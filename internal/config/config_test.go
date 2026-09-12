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
}

func TestLoadOverridesDefault(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "config.json")

	data := []byte(`{
		"server": {
			"address": "127.0.0.1:9090"
		}
	}`)

	err := os.WriteFile(path, data, 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
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

	data := []byte(`{
		"server": {
			"address": "127.0.0.1:9090"
		}
	}`)

	err := os.WriteFile(path, data, 0644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	// 修改文件，使其包含无效的 JSON
	err = os.WriteFile(path, []byte(`{ invalid json }`), 0644)
	if err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	_, err = Load(path)
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

	if !strings.Contains(err.Error(), "server address is required") {
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
