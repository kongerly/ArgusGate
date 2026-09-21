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
	p := New(client, endpoint)

	raw := []byte(`{"model":"qwen3","stream":false}`)

	resp, err := p.Forward(context.Background(), raw)
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
	p := New(client, endpoint)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	raw := []byte(`{"model":"qwen3","stream":false}`)

	errCh := make(chan error, 1)
	go func() {
		_, err := p.Forward(ctx, raw)
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
