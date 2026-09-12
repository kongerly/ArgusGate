package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Config struct {
	Server ServerConfig `json:"server"`
}

type ServerConfig struct {
	Address string `json:"address"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Address: "127.0.0.1:8080",
		},
	}
}

func (c Config) Validate() error {
	if c.Server.Address == "" {
		return fmt.Errorf("server address is required")
	}

	return nil
}

func Load(path string) (Config, error) {
	cfg := Default()

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	err = decoder.Decode(&cfg)
	if err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}

	var extra any

	err = decoder.Decode(&extra)

	// 第二次解码只用于确保文件中恰好包含一个顶层 JSON 值；尾随空白仍然合法。
	if err == nil {
		return Config{}, fmt.Errorf("parse config %q: multiple JSON values", path)
	}

	if err != io.EOF {
		return Config{}, fmt.Errorf("parse config %q: trailing data: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config %q: %w", path, err)
	}

	return cfg, nil
}
