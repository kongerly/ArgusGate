package proxy

import (
	"net/http"
	"testing"
)

func TestCopyHeaders(t *testing.T) {
	t.Run("normal headers are copied", func(t *testing.T) {
		src := http.Header{
			"Content-Type": {"application/json"},
			"Accept":       {"application/json"},
		}
		dst := http.Header{}

		copyHeaders(dst, src)

		if got := dst.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want %q", got, "application/json")
		}
		if got := dst.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want %q", got, "application/json")
		}
	})

	t.Run("fixed hop-by-hop headers are filtered", func(t *testing.T) {
		src := http.Header{
			"Connection":        {"keep-alive"},
			"Keep-Alive":        {"timeout=5"},
			"Transfer-Encoding": {"chunked"},
			"Upgrade":           {"websocket"},
			"Content-Type":      {"application/json"},
		}
		dst := http.Header{}

		copyHeaders(dst, src)

		for _, name := range []string{"Connection", "Keep-Alive", "Transfer-Encoding", "Upgrade"} {
			if _, ok := dst[name]; ok {
				t.Errorf("%s should be filtered, got %v", name, dst[name])
			}
		}
		if got := dst.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want %q", got, "application/json")
		}
	})

	t.Run("connection-declared headers are filtered", func(t *testing.T) {
		src := http.Header{
			"Connection": {"Foo, Bar"},
			"Foo":        {"foo-value"},
			"Bar":        {"bar-value"},
			"X-Keep":     {"keep-value"},
		}
		dst := http.Header{}

		copyHeaders(dst, src)

		if _, ok := dst["Foo"]; ok {
			t.Errorf("Foo should be filtered, got %v", dst["Foo"])
		}
		if _, ok := dst["Bar"]; ok {
			t.Errorf("Bar should be filtered, got %v", dst["Bar"])
		}
		if got := dst.Get("X-Keep"); got != "keep-value" {
			t.Errorf("X-Keep = %q, want %q", got, "keep-value")
		}
	})

	t.Run("multiple Connection headers are parsed", func(t *testing.T) {
		src := http.Header{
			"Connection": {"Foo", "Bar, Baz"},
			"Foo":        {"foo-value"},
			"Bar":        {"bar-value"},
			"Baz":        {"baz-value"},
			"X-Keep":     {"keep-value"},
		}
		dst := http.Header{}

		copyHeaders(dst, src)

		for _, name := range []string{"Foo", "Bar", "Baz"} {
			if _, ok := dst[name]; ok {
				t.Errorf("%s should be filtered, got %v", name, dst[name])
			}
		}
		if got := dst.Get("X-Keep"); got != "keep-value" {
			t.Errorf("X-Keep = %q, want %q", got, "keep-value")
		}
	})

	t.Run("managed headers are filtered", func(t *testing.T) {
		src := http.Header{
			"Authorization": {"Bearer client-token"},
			"X-Request-Id":  {"client-id"},
			"Accept":        {"application/json"},
		}
		dst := http.Header{}

		copyHeaders(dst, src)

		if _, ok := dst["Authorization"]; ok {
			t.Errorf("Authorization should be filtered, got %v", dst["Authorization"])
		}
		if _, ok := dst["X-Request-Id"]; ok {
			t.Errorf("X-Request-Id should be filtered, got %v", dst["X-Request-Id"])
		}
		if got := dst.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want %q", got, "application/json")
		}
	})

	t.Run("multi-value headers are preserved", func(t *testing.T) {
		src := http.Header{
			"X-Test": {"one", "two"},
		}
		dst := http.Header{}

		copyHeaders(dst, src)

		got := dst.Values("X-Test")
		if len(got) != 2 {
			t.Fatalf("X-Test has %d values, want 2: %v", len(got), got)
		}
		if got[0] != "one" || got[1] != "two" {
			t.Errorf("X-Test = %v, want [one two]", got)
		}
	})
}

func TestCopyResponseHeaders(t *testing.T) {
	t.Run("end-to-end headers are copied", func(t *testing.T) {
		src := http.Header{
			"Content-Type": {"application/json"},
			"Retry-After":  {"3"},
		}
		dst := http.Header{}

		copyResponseHeaders(dst, src)

		if got := dst.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want %q", got, "application/json")
		}
		if got := dst.Get("Retry-After"); got != "3" {
			t.Errorf("Retry-After = %q, want %q", got, "3")
		}
	})

	t.Run("fixed hop-by-hop headers are filtered", func(t *testing.T) {
		src := http.Header{
			"Connection":        {"keep-alive"},
			"Keep-Alive":        {"timeout=5"},
			"Transfer-Encoding": {"chunked"},
			"Upgrade":           {"websocket"},
			"Content-Type":      {"application/json"},
		}
		dst := http.Header{}

		copyResponseHeaders(dst, src)

		for _, name := range []string{"Connection", "Keep-Alive", "Transfer-Encoding", "Upgrade"} {
			if _, ok := dst[name]; ok {
				t.Errorf("%s should be filtered, got %v", name, dst[name])
			}
		}
		if got := dst.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want %q", got, "application/json")
		}
	})

	t.Run("connection-declared headers are filtered", func(t *testing.T) {
		src := http.Header{
			"Connection": {"Foo, Bar"},
			"Foo":        {"foo-value"},
			"Bar":        {"bar-value"},
			"X-Keep":     {"keep-value"},
		}
		dst := http.Header{}

		copyResponseHeaders(dst, src)

		if _, ok := dst["Foo"]; ok {
			t.Errorf("Foo should be filtered, got %v", dst["Foo"])
		}
		if _, ok := dst["Bar"]; ok {
			t.Errorf("Bar should be filtered, got %v", dst["Bar"])
		}
		if got := dst.Get("X-Keep"); got != "keep-value" {
			t.Errorf("X-Keep = %q, want %q", got, "keep-value")
		}
	})

	t.Run("X-Request-ID from backend is filtered", func(t *testing.T) {
		src := http.Header{
			"X-Request-Id": {"backend-id"},
			"Content-Type": {"application/json"},
		}
		dst := http.Header{}

		copyResponseHeaders(dst, src)

		if _, ok := dst["X-Request-Id"]; ok {
			t.Errorf("X-Request-Id should be filtered, got %v", dst["X-Request-Id"])
		}
		if got := dst.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want %q", got, "application/json")
		}
	})

	t.Run("Authorization from backend is preserved", func(t *testing.T) {
		src := http.Header{
			"Authorization": {"Bearer backend-token"},
		}
		dst := http.Header{}

		copyResponseHeaders(dst, src)

		if got := dst.Get("Authorization"); got != "Bearer backend-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer backend-token")
		}
	})

	t.Run("multi-value headers are preserved", func(t *testing.T) {
		src := http.Header{
			"Set-Cookie": {
				"session=abc; Path=/",
				"theme=dark; Path=/",
			},
		}
		dst := http.Header{}

		copyResponseHeaders(dst, src)

		got := dst.Values("Set-Cookie")
		if len(got) != 2 {
			t.Fatalf("Set-Cookie has %d values, want 2: %v", len(got), got)
		}
		if got[0] != "session=abc; Path=/" || got[1] != "theme=dark; Path=/" {
			t.Errorf("Set-Cookie = %v, want [session=abc; Path=/ theme=dark; Path=/]", got)
		}
	})

	t.Run("non-canonical connection key", func(t *testing.T) {
		src := http.Header{
			"connection": {"Foo"},
			"Foo":        {"should-not-pass"},
			"X-Keep":     {"keep-value"},
		}
		dst := http.Header{}

		copyResponseHeaders(dst, src)

		if _, ok := dst["Foo"]; ok {
			t.Errorf("Foo should be filtered, got %v", dst["Foo"])
		}
		if got := dst.Get("X-Keep"); got != "keep-value" {
			t.Errorf("X-Keep = %q, want %q", got, "keep-value")
		}
	})
}
