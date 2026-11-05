package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kongerly/ArgusGate/internal/proxy"
)

func chatTestHandler(t *testing.T, client *http.Client, endpoint, authorization string, logs io.Writer) http.Handler {
	t.Helper()
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	return NewHandler(slog.New(slog.NewTextHandler(logs, nil)), proxy.New(client, u, authorization))
}

func TestChatCompletionsForwarding(t *testing.T) {
	for _, status := range []int{200, 429, 500} {
		for _, authorization := range []string{"", "Bearer backend-secret"} {
			t.Run(fmt.Sprintf("status_%d/credential_%t", status, authorization != ""), func(t *testing.T) {
				raw := " {\n  \"model\":\"unmapped-model\", \"stream\":false, \"unknown\":1.2300e+02, \"messages\":null\n}\n"
				responseBody := fmt.Sprintf(`{"upstream_status":%d}`, status)
				type captured struct {
					body, method, host string
					header             http.Header
				}
				received := make(chan captured, 1)
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Errorf("read upstream body: %v", err)
					}
					received <- captured{string(body), r.Method, r.Host, r.Header.Clone()}
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Retry-After", "3")
					w.Header().Set(requestIDHeader, "backend-id")
					w.Header().Set("Connection", "X-Private")
					w.Header().Set("X-Private", "drop")
					w.WriteHeader(status)
					_, _ = io.WriteString(w, responseBody)
				}))
				t.Cleanup(upstream.Close)
				handler := chatTestHandler(t, upstream.Client(), upstream.URL+"/v1/chat/completions", authorization, io.Discard)
				gateway := httptest.NewServer(handler)
				t.Cleanup(gateway.Close)
				req, err := http.NewRequest(http.MethodPost, gateway.URL+"/v1/chat/completions", strings.NewReader(raw))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Content-Type", `application/json; charset="utf-8"`)
				req.Header.Set("Accept", "application/json")
				req.Header.Set("Authorization", "Bearer client-secret")
				req.Header.Set(requestIDHeader, "client-id")
				req.Header.Set("Connection", "X-Remove")
				req.Header.Set("X-Remove", "drop")
				resp, err := gateway.Client().Do(req)
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}
				if resp.StatusCode != status || string(body) != responseBody {
					t.Fatalf("response = %d %q", resp.StatusCode, body)
				}
				got := <-received
				if got.body != raw || got.method != http.MethodPost {
					t.Fatalf("upstream request = %s %q", got.method, got.body)
				}
				u, _ := url.Parse(upstream.URL)
				if got.host != u.Host {
					t.Errorf("upstream host = %q", got.host)
				}
				id := resp.Header.Get(requestIDHeader)
				if id == "" || id == "client-id" || id == "backend-id" || got.header.Get(requestIDHeader) != id {
					t.Errorf("inconsistent gateway request ID: response=%q upstream=%q", id, got.header.Get(requestIDHeader))
				}
				if got.header.Get("Authorization") != authorization {
					t.Error("upstream authorization mismatch")
				}
				if got.header.Get("Content-Type") != req.Header.Get("Content-Type") || got.header.Get("Accept") != "application/json" || got.header.Get("X-Remove") != "" {
					t.Errorf("upstream headers = %v", got.header)
				}
				if resp.Header.Get("Content-Type") != "application/json" || resp.Header.Get("Retry-After") != "3" || resp.Header.Get("X-Private") != "" {
					t.Errorf("response headers = %v", resp.Header)
				}
			})
		}
	}
}

func assertChatError(t *testing.T, rec *httptest.ResponseRecorder, status int, errorType, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d: %s", rec.Code, status, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q", rec.Header().Get("Content-Type"))
	}
	var response errorResponse
	decoder := json.NewDecoder(rec.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != code || response.Error.Type != errorType || response.Error.Message == "" || response.Error.RequestID == "" || response.Error.RequestID != rec.Header().Get(requestIDHeader) {
		t.Fatalf("invalid error envelope: %+v", response)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		t.Fatalf("extra error content: %v", err)
	}
}

func TestChatCompletionsInvalidRequests(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer upstream.Close()
	handler := chatTestHandler(t, upstream.Client(), upstream.URL, "", io.Discard)
	cases := []struct {
		name, method, contentType, body string
		status                          int
		code                            string
	}{
		{"get", "GET", "application/json", `{}`, 405, "method_not_allowed"},
		{"head", "HEAD", "application/json", `{}`, 405, "method_not_allowed"},
		{"put", "PUT", "application/json", `{}`, 405, "method_not_allowed"},
		{"delete", "DELETE", "application/json", `{}`, 405, "method_not_allowed"},
		{"options", "OPTIONS", "application/json", `{}`, 405, "method_not_allowed"},
		{"missing content type", "POST", "", `{}`, 415, "unsupported_media_type"},
		{"text", "POST", "text/plain", `{}`, 415, "unsupported_media_type"},
		{"invalid parameter", "POST", "application/json; charset", `{}`, 415, "unsupported_media_type"},
		{"too large", "POST", "application/json", strings.Repeat(" ", (4<<20)+1), 413, "request_too_large"},
		{"empty", "POST", "application/json", "", 400, "invalid_json"},
		{"whitespace", "POST", "application/json", " \n", 400, "invalid_json"},
		{"malformed", "POST", "application/json", `{`, 400, "invalid_json"},
		{"trailing JSON", "POST", "application/json", `{} {}`, 400, "invalid_json"},
		{"array", "POST", "application/json", `[]`, 400, "invalid_json"},
		{"null", "POST", "application/json", `null`, 400, "invalid_json"},
		{"model type", "POST", "application/json", `{"model":123}`, 400, "invalid_json"},
		{"stream type", "POST", "application/json", `{"model":"m","stream":"true"}`, 400, "invalid_json"},
		{"missing model", "POST", "application/json", `{}`, 400, "missing_model"},
		{"null model", "POST", "application/json", `{"model":null}`, 400, "missing_model"},
		{"empty model", "POST", "application/json", `{"model":""}`, 400, "missing_model"},
		{"blank model", "POST", "application/json", `{"model":"  "}`, 400, "missing_model"},
		{"stream", "POST", "application/json", `{"model":"m","stream":true}`, 400, "stream_not_supported"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/v1/chat/completions", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if tc.status == 405 && rec.Header().Get("Allow") != "POST" {
				t.Error("missing Allow: POST")
			}
			assertChatError(t, rec, tc.status, "invalid_request_error", tc.code)
			if calls.Load() != 0 {
				t.Fatal("invalid request reached upstream")
			}
		})
	}
}

type chatRoundTripper func(*http.Request) (*http.Response, error)

func (f chatRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestChatCompletionsBodyLimitAndMediaType(t *testing.T) {
	for _, contentType := range []string{"application/json", "Application/JSON; charset=UTF-8", `application/json; charset="utf-8"`} {
		t.Run(contentType, func(t *testing.T) {
			raw := `{"model":"m"}`
			raw += strings.Repeat(" ", (4<<20)-len(raw))
			client := &http.Client{Transport: chatRoundTripper(func(r *http.Request) (*http.Response, error) {
				body, _ := io.ReadAll(r.Body)
				if string(body) != raw {
					t.Error("body at size limit changed")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Header: http.Header{}}, nil
			})}
			handler := chatTestHandler(t, client, "http://backend/v1/chat/completions", "", io.Discard)
			req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(raw))
			req.Header.Set("Content-Type", contentType)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != 200 || rec.Body.String() != "ok" {
				t.Fatalf("response = %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestChatCompletionsForwardFailure(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(fmt.Sprint(canceled), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := &http.Client{Transport: chatRoundTripper(func(r *http.Request) (*http.Response, error) {
				if r.Context().Value(chatContextKey{}) != "inherited" {
					t.Error("request context not inherited")
				}
				if canceled {
					cancel()
					return nil, r.Context().Err()
				}
				return nil, errors.New("unsafe backend-secret http://private-backend/")
			})}
			var logs bytes.Buffer
			handler := chatTestHandler(t, client, "http://backend/v1/chat/completions", "", &logs)
			req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"m"}`))
			req = req.WithContext(context.WithValue(ctx, chatContextKey{}, "inherited"))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			observed := &responseWriter{ResponseWriter: rec}
			handler.ServeHTTP(observed, req)
			if canceled {
				if observed.wroteHeader || rec.Body.Len() != 0 {
					t.Fatal("wrote error after cancellation")
				}
			} else {
				if strings.Contains(rec.Body.String(), "backend-secret") || strings.Contains(rec.Body.String(), "private-backend") {
					t.Fatal("unsafe error returned to client")
				}
				assertChatError(t, rec, 502, "upstream_error", "upstream_connect_error")
			}
			if strings.Contains(logs.String(), "backend-secret") || strings.Contains(logs.String(), "private-backend") {
				t.Fatal("unsafe error logged")
			}
		})
	}
}

type chatContextKey struct{}

func TestChatCompletionsBodyReadFailure(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(fmt.Sprint(canceled), func(t *testing.T) {
			client := &http.Client{Transport: chatRoundTripper(func(r *http.Request) (*http.Response, error) {
				t.Error("body read failure reached upstream")
				return nil, errors.New("unexpected upstream call")
			})}
			handler := chatTestHandler(t, client, "http://backend/v1/chat/completions", "", io.Discard)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if canceled {
				cancel()
			}
			req := httptest.NewRequest("POST", "/v1/chat/completions", nil).WithContext(ctx)
			req.Body = io.NopCloser(errorReader{err: errors.New("unsafe internal error")})
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			observed := &responseWriter{ResponseWriter: rec}
			handler.ServeHTTP(observed, req)
			if canceled {
				if observed.wroteHeader || rec.Body.Len() != 0 {
					t.Fatal("wrote error after cancellation")
				}
			} else {
				assertChatError(t, rec, 400, "invalid_request_error", "invalid_json")
			}
		})
	}
}

type chatFailingBody struct{ closed bool }

func (b *chatFailingBody) Read(p []byte) (int, error) {
	return copy(p, "partial"), errors.New("unsafe body error")
}
func (b *chatFailingBody) Close() error { b.closed = true; return nil }

func TestChatCompletionsPostCommitFailure(t *testing.T) {
	body := &chatFailingBody{}
	client := &http.Client{Transport: chatRoundTripper(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 429, Header: http.Header{"Content-Type": {"application/json"}}, Body: body}, nil
	})}
	var logs bytes.Buffer
	handler := chatTestHandler(t, client, "http://backend/v1/chat/completions", "", &logs)
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"m"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 429 || rec.Body.String() != "partial" || !body.closed {
		t.Fatalf("post-commit response = %d %q, closed=%t", rec.Code, rec.Body.String(), body.closed)
	}
	if !strings.Contains(logs.String(), "response forwarding failed") {
		t.Fatal("missing post-commit error log")
	}
}
