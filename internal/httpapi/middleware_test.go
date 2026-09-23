package httpapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kongerly/ArgusGate/internal/proxy"
)

func newTestProxy(t *testing.T) *proxy.Proxy {
	t.Helper()

	endpoint, err := url.Parse("http://127.0.0.1:0")
	if err != nil {
		t.Fatalf("parse test endpoint: %v", err)
	}

	return proxy.New(http.DefaultClient, endpoint, "")
}

func TestRequestIDMiddlewareAddsRequestIDToContext(t *testing.T) {
	var gotRequestID string

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestID = requestIDFromContext(r.Context())
	})

	handler := requestIDMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if gotRequestID == "" {
		t.Fatal("expected request ID in context, got empty value")
	}

	if rec.Header().Get(requestIDHeader) != gotRequestID {
		t.Fatalf(
			"expected response request ID %q, got %q",
			gotRequestID,
			rec.Header().Get(requestIDHeader),
		)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	var buf bytes.Buffer

	logger := slog.New(
		slog.NewTextHandler(&buf, nil),
	)

	handler := NewHandler(logger, newTestProxy(t))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()

	if !strings.Contains(logOutput, "method=GET") {
		t.Fatalf("expected method in log, got %q", logOutput)
	}

	if !strings.Contains(logOutput, "path=/healthz") {
		t.Fatalf("expected path in log, got %q", logOutput)
	}

	if !strings.Contains(logOutput, "status=200") {
		t.Fatalf("expected status in log, got %q", logOutput)
	}

	if !strings.Contains(logOutput, "request_id=") {
		t.Fatalf("expected request ID in log, got %q", logOutput)
	}

	if !strings.Contains(logOutput, "duration=") {
		t.Fatalf("expected duration in log, got %q", logOutput)
	}
}
