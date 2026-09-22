package proxy

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestForwardSendsPostAndPreservesBody(t *testing.T) {
	type capturedRequest struct {
		method string
		body   string
	}
	received := make(chan capturedRequest, 1)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read backend body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		received <- capturedRequest{
			method: r.Method,
			body:   string(body),
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	endpoint, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatalf("parse backend url: %v", err)
	}

	client := &http.Client{}
	p := New(client, endpoint, "")

	raw := []byte(`{"model":"qwen3","stream":false}`)

	resp, err := p.Forward(context.Background(), raw, http.Header{}, "")
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}
	defer resp.Body.Close()

	got := <-received

	if got.method != http.MethodPost {
		t.Errorf("method = %q, want %q", got.method, http.MethodPost)
	}

	if got.body != string(raw) {
		t.Errorf("body = %q, want %q", got.body, string(raw))
	}
}

func TestForwardStopsWhenContextCanceled(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}

		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer backend.Close()
	defer close(release)

	endpoint, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatalf("parse backend url: %v", err)
	}

	client := &http.Client{}
	p := New(client, endpoint, "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	raw := []byte(`{"model":"qwen3","stream":false}`)

	errCh := make(chan error, 1)
	go func() {
		_, err := p.Forward(ctx, raw, http.Header{}, "")
		errCh <- err
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("backend did not receive request")
	}

	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Forward did not return after context canceled")
	}
}

func TestForwardFiltersAndSetsHeaders(t *testing.T) {
	type capturedHeaders struct {
		host          string
		contentType   string
		accept        string
		connection    string
		foo           string
		authorization string
		requestID     string
	}
	received := make(chan capturedHeaders, 1)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- capturedHeaders{
			host:          r.Host,
			contentType:   r.Header.Get("Content-Type"),
			accept:        r.Header.Get("Accept"),
			connection:    r.Header.Get("Connection"),
			foo:           r.Header.Get("Foo"),
			authorization: r.Header.Get("Authorization"),
			requestID:     r.Header.Get("X-Request-ID"),
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	endpoint, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatalf("parse backend url: %v", err)
	}

	client := &http.Client{}
	p := New(client, endpoint, "Bearer backend-secret")

	incomingHeaders := http.Header{
		"Content-Type":  {"application/json"},
		"Accept":        {"application/json"},
		"Connection":    {"Foo"},
		"Foo":           {"should-not-pass"},
		"Authorization": {"Bearer client-secret"},
		"X-Request-Id":  {"fake-client-id"},
	}

	raw := []byte(`{"model":"qwen3","stream":false}`)

	resp, err := p.Forward(
		context.Background(),
		raw,
		incomingHeaders,
		"argusgate-id",
	)
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}
	defer resp.Body.Close()

	got := <-received

	// Host 必须是 backend 自己的 host
	if got.host != endpoint.Host {
		t.Errorf("Host = %q, want %q", got.host, endpoint.Host)
	}

	// 普通 end-to-end headers 被转发
	if got.contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got.contentType, "application/json")
	}
	if got.accept != "application/json" {
		t.Errorf("Accept = %q, want %q", got.accept, "application/json")
	}

	// hop-by-hop 和 Connection 动态声明的 header 被过滤
	if got.connection != "" {
		t.Errorf("Connection should be filtered, got %q", got.connection)
	}
	if got.foo != "" {
		t.Errorf("Foo should be filtered, got %q", got.foo)
	}

	// 客户端 Authorization 被过滤，backend Authorization 由 Proxy 设置
	if got.authorization != "Bearer backend-secret" {
		t.Errorf("Authorization = %q, want %q", got.authorization, "Bearer backend-secret")
	}

	// X-Request-ID 是 ArgusGate 设置的值，不是客户端的
	if got.requestID != "argusgate-id" {
		t.Errorf("X-Request-ID = %q, want %q", got.requestID, "argusgate-id")
	}
}

func TestForwardDoesNotLeakManagedHeaders(t *testing.T) {
	type capturedHeaders struct {
		authorization string
		requestID     string
	}
	received := make(chan capturedHeaders, 1)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- capturedHeaders{
			authorization: r.Header.Get("Authorization"),
			requestID:     r.Header.Get("X-Request-ID"),
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	endpoint, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatalf("parse backend url: %v", err)
	}

	client := &http.Client{}

	// 关键：backend 没有配置 credential
	p := New(client, endpoint, "")

	incomingHeaders := http.Header{
		"Authorization": {"Bearer client-secret"},
		"X-Request-Id":  {"fake-client-id"},
	}

	raw := []byte(`{"model":"qwen3","stream":false}`)

	// 关键：requestID 传空字符串
	resp, err := p.Forward(
		context.Background(),
		raw,
		incomingHeaders,
		"",
	)
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}
	defer resp.Body.Close()

	got := <-received

	if got.authorization != "" {
		t.Errorf("Authorization leaked: got %q, want empty", got.authorization)
	}
	if got.requestID != "" {
		t.Errorf("X-Request-ID leaked: got %q, want empty", got.requestID)
	}
}
