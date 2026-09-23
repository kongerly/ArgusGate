package httpapi

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/kongerly/ArgusGate/internal/proxy"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()

	endpoint, err := url.Parse("http://127.0.0.1:0")
	if err != nil {
		t.Fatalf("parse test endpoint: %v", err)
	}

	p := proxy.New(http.DefaultClient, endpoint, "")

	return NewHandler(slog.Default(), p)
}

func TestHealthz(t *testing.T) {
	handler := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHealthzRejectsPost(t *testing.T) {
	handler := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusMethodNotAllowed,
			rec.Code,
		)
	}

	requestID := rec.Header().Get(requestIDHeader)
	if requestID == "" {
		t.Fatal("expected request ID header, got empty value")
	}
}
