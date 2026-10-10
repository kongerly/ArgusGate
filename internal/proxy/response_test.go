package proxy

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteResponsePreservesStatusAndBody(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		header     http.Header
		body       string
	}{
		{
			name:       "200 with JSON body",
			statusCode: http.StatusOK,
			header: http.Header{
				"Content-Type": {"application/json"},
				"X-Request-Id": {"backend-id"},
			},
			body: `{"result":"ok"}`,
		},
		{
			name:       "429 with Retry-After",
			statusCode: http.StatusTooManyRequests,
			header: http.Header{
				"Content-Type": {"application/json"},
				"Retry-After":  {"3"},
				"X-Request-Id": {"backend-id"},
			},
			body: `{"error":"rate limited"}`,
		},
		{
			name:       "500 with JSON body",
			statusCode: http.StatusInternalServerError,
			header: http.Header{
				"Content-Type": {"application/json"},
				"X-Request-Id": {"backend-id"},
			},
			body: `{"error":"internal"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Header:     tt.header,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}

			rec := httptest.NewRecorder()
			rec.Header().Set("X-Request-Id", "argusgate-id")

			if err := WriteResponse(rec, resp); err != nil {
				t.Fatalf("WriteResponse() error = %v", err)
			}

			result := rec.Result()
			defer result.Body.Close()

			if result.StatusCode != tt.statusCode {
				t.Errorf("status = %d, want %d", result.StatusCode, tt.statusCode)
			}

			if got := result.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want %q", got, "application/json")
			}

			if want := tt.header.Get("Retry-After"); want != "" {
				if got := result.Header.Get("Retry-After"); got != want {
					t.Errorf("Retry-After = %q, want %q", got, want)
				}
			}

			body, err := io.ReadAll(result.Body)
			if err != nil {
				t.Fatalf("read result body: %v", err)
			}
			if string(body) != tt.body {
				t.Errorf("body = %q, want %q", string(body), tt.body)
			}

			if got := result.Header.Get("X-Request-Id"); got != "argusgate-id" {
				t.Errorf("X-Request-Id = %q, want %q", got, "argusgate-id")
			}
		})
	}
}

type trackingReadCloser struct {
	reader io.Reader
	closed bool
}

func (t *trackingReadCloser) Read(p []byte) (int, error) {
	return t.reader.Read(p)
}

func (t *trackingReadCloser) Close() error {
	t.closed = true
	return nil
}

func TestWriteResponseClosesBody(t *testing.T) {
	body := &trackingReadCloser{
		reader: strings.NewReader(`{"result":"ok"}`),
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       body,
	}

	rec := httptest.NewRecorder()

	if err := WriteResponse(rec, resp); err != nil {
		t.Fatalf("WriteResponse() error = %v", err)
	}

	if !body.closed {
		t.Fatal("resp.Body was not closed")
	}
}

type failingReader struct {
	sentinel error
	readOnce bool
}

func (f *failingReader) Read(p []byte) (int, error) {
	if !f.readOnce {
		f.readOnce = true
		n := copy(p, "partial")
		return n, nil
	}
	return 0, f.sentinel
}

func TestWriteResponseClosesBodyOnReadError(t *testing.T) {
	sentinel := errors.New("boom")

	body := &trackingReadCloser{
		reader: &failingReader{sentinel: sentinel},
	}

	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       body,
	}

	rec := httptest.NewRecorder()

	err := WriteResponse(rec, resp)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, sentinel) {
		t.Fatalf("expected error chain to contain %v, got %v", sentinel, err)
	}

	if !body.closed {
		t.Fatal("resp.Body was not closed on error path")
	}

	// 响应已经 commit：状态码和部分 body 已经发给客户端。
	result := rec.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", result.StatusCode, http.StatusTooManyRequests)
	}

	if got := rec.Body.String(); got != "partial" {
		t.Errorf("body = %q, want %q", got, "partial")
	}
}
