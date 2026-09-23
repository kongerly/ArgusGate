package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Server.Address != "127.0.0.1:8080" {
		t.Fatalf(
			"expected default address %q, got %q",
			"127.0.0.1:8080",
			cfg.Server.Address,
		)
	}

	if cfg.Server.ShutdownTimeout != "5s" {
		t.Fatalf(
			"expected default shutdown timeout %q, got %q",
			"5s",
			cfg.Server.ShutdownTimeout,
		)
	}
}

func TestValidateBackendRequirements(t *testing.T) {
	tests := []struct {
		name        string
		backends    []BackendConfig
		wantErrText string
	}{
		{
			name:        "nil backends",
			backends:    nil,
			wantErrText: "exactly one backend is required",
		},
		{
			name: "two backends",
			backends: []BackendConfig{
				{Origin: "http://127.0.0.1:8081"},
				{Origin: "http://127.0.0.1:8082"},
			},
			wantErrText: "exactly one backend is required",
		},
		{
			name: "empty origin",
			backends: []BackendConfig{
				{Origin: ""},
			},
			wantErrText: "validate backend origin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Backends = tt.backends

			err := cfg.Validate()
			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantErrText) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErrText, err.Error())
			}
		})
	}
}

func TestLoadOverridesDefault(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "config.json")

	data := []byte(`{
		"server": {
			"address": "127.0.0.1:9090"
		}
	}`)

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Address != "127.0.0.1:9090" {
		t.Fatalf(
			"expected address %q, got %q",
			"127.0.0.1:9090",
			cfg.Server.Address,
		)
	}

	// 未显式配置 shutdown_timeout 时保留默认值，确保旧的最小配置仍可使用。
	if cfg.Server.ShutdownTimeout != "5s" {
		t.Fatalf(
			"expected default shutdown timeout %q, got %q",
			"5s",
			cfg.Server.ShutdownTimeout,
		)
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := Load("./does-not-exist.json")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if !strings.Contains(err.Error(), "read config") {
		t.Fatalf("expected read config error, got %q", err.Error())
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "config.json")

	data := []byte(`{ invalid json }`)

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("expected parse config error, got %q", err.Error())
	}
}

func TestLoadUnknownField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	data := []byte(`{
		"server": {
			"address": "127.0.0.1:8080",
			"adress": "oops"
		}
	}`)

	err := os.WriteFile(path, data, 0644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err = Load(path)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %q", err.Error())
	}
}

func TestLoadInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	data := []byte(`{
		"server": {
			"address": ""
		}
	}`)

	err := os.WriteFile(path, data, 0644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err = Load(path)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if !strings.Contains(err.Error(), "validate config") {
		t.Fatalf("expected validate config error, got %q", err.Error())
	}

	if !strings.Contains(err.Error(), "server.address must not be empty") {
		t.Fatalf("expected server address validation error, got %q", err.Error())
	}
}

func TestLoadWithoutPath(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Address != "127.0.0.1:8080" {
		t.Fatalf(
			"expected default address %q, got %q",
			"127.0.0.1:8080",
			cfg.Server.Address,
		)
	}

	if cfg.Server.ShutdownTimeout != "5s" {
		t.Fatalf(
			"expected default shutdown timeout %q, got %q",
			"5s",
			cfg.Server.ShutdownTimeout,
		)
	}
}

func TestLoadMultipleJSONValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	data := []byte(`{
		"server": {
			"address": "127.0.0.1:8080"
		}
	}

	{
		"server": {
			"address": "0.0.0.0:9000"
		}
	}`)

	err := os.WriteFile(path, data, 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Load(path)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if !strings.Contains(err.Error(), "multiple JSON values") {
		t.Fatalf(
			"expected multiple JSON values error, got %q",
			err.Error(),
		)
	}
}

func TestLoadEmptyShutdownTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	data := []byte(`{
		"server": {
			"address": "127.0.0.1:8080",
			"shutdown_timeout": ""
		}
	}`)

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if !strings.Contains(err.Error(), "validate config") {
		t.Fatalf("expected validate config error, got %q", err.Error())
	}

	if !strings.Contains(err.Error(), "shutdown_timeout must not be empty") {
		t.Fatalf("expected shutdown timeout empty error, got %q", err.Error())
	}
}

func TestLoadInvalidShutdownTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	data := []byte(`{
		"server": {
			"address": "127.0.0.1:8080",
			"shutdown_timeout": "not-a-duration"
		}
	}`)

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if !strings.Contains(err.Error(), "validate config") {
		t.Fatalf("expected validate config error, got %q", err.Error())
	}

	if !strings.Contains(err.Error(), "must be a valid duration") {
		t.Fatalf("expected invalid duration error, got %q", err.Error())
	}
}

func TestValidateBackendOrigin(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		wantErr string
	}{
		// 合法
		{name: "http with port", origin: "http://127.0.0.1:8081"},
		{name: "https", origin: "https://backend.example.com"},
		{name: "trailing slash", origin: "http://backend:8081/"},
		{name: "uppercase scheme", origin: "HTTP://backend:8081"},
		{name: "mixed-case scheme", origin: "Https://backend.example.com"},

		// 空
		{name: "empty", origin: "", wantErr: "must not be empty"},
		{name: "whitespace only", origin: "   ", wantErr: "must not be empty"},

		// scheme
		{name: "missing scheme", origin: "backend:8081", wantErr: "scheme"},
		{name: "ftp scheme", origin: "ftp://backend:8081", wantErr: "scheme"},

		// host
		{name: "empty host", origin: "http:///foo", wantErr: "host"},

		// userinfo
		{name: "userinfo", origin: "http://user:pass@backend:8081", wantErr: "userinfo"},

		// path
		{name: "business path", origin: "http://backend:8081/api", wantErr: "path"},

		// query
		{name: "query with value", origin: "http://backend:8081?foo=bar", wantErr: "query"},
		{name: "empty query with question mark", origin: "http://backend:8081?", wantErr: "query"},

		// fragment
		{name: "fragment", origin: "http://backend:8081#foo", wantErr: "fragment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBackendOrigin(tt.origin)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestValidateDelegatesToValidateBackendOrigin(t *testing.T) {
	cfg := Default()
	cfg.Backends[0].Origin = "ftp://backend:8081"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if !strings.Contains(err.Error(), "validate backend origin") {
		t.Fatalf("expected error to mention backend origin context, got %q", err.Error())
	}

	if !strings.Contains(err.Error(), "scheme must be http or https") {
		t.Fatalf("expected error to mention scheme rule, got %q", err.Error())
	}
}
