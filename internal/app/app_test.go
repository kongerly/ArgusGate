package app

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/kongerly/ArgusGate/internal/config"
)

func TestRunStopsWhenContextCanceled(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Address = "127.0.0.1:0"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, cfg)
	}()

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context canceled")
	}
}

func TestRunReturnsErrorWhenAddressInUse(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()

	cfg := config.Default()
	cfg.Server.Address = addr

	ctx := context.Background()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, cfg)
	}()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error when address in use, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s")
	}
}

func TestBuildBackendEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		want    string
		wantErr bool
	}{
		{
			name:   "no trailing slash",
			origin: "http://backend:8081",
			want:   "http://backend:8081/v1/chat/completions",
		},
		{
			name:   "with trailing slash",
			origin: "http://backend:8081/",
			want:   "http://backend:8081/v1/chat/completions",
		},
		{
			name:    "parse error",
			origin:  "://invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildBackendEndpoint(tt.origin)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.String() != tt.want {
				t.Fatalf("endpoint = %q, want %q", got.String(), tt.want)
			}
		})
	}
}
