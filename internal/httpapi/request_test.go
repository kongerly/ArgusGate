package httpapi

import (
	"errors"
	"strings"
	"testing"
)

func TestReadBody(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		maxSize int64
		want    string
		wantErr error
	}{
		{
			name:    "smaller than max",
			body:    "abc",
			maxSize: 5,
			want:    "abc",
		},
		{
			name:    "exactly max",
			body:    "abcde",
			maxSize: 5,
			want:    "abcde",
		},
		{
			name:    "larger than max",
			body:    "abcdef",
			maxSize: 5,
			wantErr: errBodyTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := readBody(strings.NewReader(tt.body), tt.maxSize)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if string(raw) != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, string(raw))
			}
		})
	}
}

type errorReader struct {
	err error
}

func (r errorReader) Read(p []byte) (int, error) {
	return 0, r.err
}

func TestReadBodyReaderError(t *testing.T) {
	sentinel := errors.New("boom")

	_, err := readBody(errorReader{err: sentinel}, 5)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if !errors.Is(err, sentinel) {
		t.Fatalf("expected error chain to contain %v, got %v", sentinel, err)
	}
}

func TestParseRequestProbe(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    requestProbe
		wantErr bool
	}{
		{
			name: "model and stream",
			raw:  `{"model":"qwen3","stream":true}`,
			want: requestProbe{Model: "qwen3", Stream: true},
		},
		{
			name: "model only, stream defaults to false",
			raw:  `{"model":"qwen3"}`,
			want: requestProbe{Model: "qwen3", Stream: false},
		},
		{
			name: "empty object",
			raw:  `{}`,
			want: requestProbe{},
		},
		{
			name: "unknown fields ignored",
			raw:  `{"model":"qwen3","stream":false,"temperature":0.7,"messages":[{"role":"user","content":"hello"}]}`,
			want: requestProbe{Model: "qwen3", Stream: false},
		},
		{
			name: "leading whitespace",
			raw:  "  \n\t{\"model\":\"qwen3\",\"stream\":true}",
			want: requestProbe{Model: "qwen3", Stream: true},
		},
		{
			name:    "invalid json",
			raw:     `{"model":`,
			wantErr: true,
		},
		{
			name:    "multiple json values",
			raw:     `{"model":"qwen3"}{"model":"another"}`,
			wantErr: true,
		},
		{
			name:    "model wrong type",
			raw:     `{"model":123}`,
			wantErr: true,
		},
		{
			name:    "stream wrong type",
			raw:     `{"model":"qwen3","stream":"yes"}`,
			wantErr: true,
		},
		{
			name:    "empty input",
			raw:     ``,
			wantErr: true,
		},
		{
			name:    "not an object",
			raw:     `[1,2,3]`,
			wantErr: true,
		},
		{
			name:    "null",
			raw:     `null`,
			wantErr: true,
		},
		{
			name:    "string",
			raw:     `"hello"`,
			wantErr: true,
		},
		{
			name:    "number",
			raw:     `123`,
			wantErr: true,
		},
		{
			name:    "bool",
			raw:     `true`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRequestProbe([]byte(tt.raw))

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValidateProbe(t *testing.T) {
	tests := []struct {
		name    string
		probe   requestProbe
		wantErr error
	}{
		{
			name:  "valid model and stream false",
			probe: requestProbe{Model: "qwen3", Stream: false},
		},
		{
			name:    "empty model",
			probe:   requestProbe{Model: "", Stream: false},
			wantErr: errInvalidModel,
		},
		{
			name:    "whitespace-only model",
			probe:   requestProbe{Model: "   ", Stream: false},
			wantErr: errInvalidModel,
		},
		{
			name:    "stream true",
			probe:   requestProbe{Model: "qwen3", Stream: true},
			wantErr: errStreamNotSupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProbe(tt.probe)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
