package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Config 汇总启动时读取的静态配置；加载完成后调用方应将其视为不可变值。
type Config struct {
	Server ServerConfig `json:"server"`
}

// ServerConfig 定义 HTTP Server 当前阶段实际使用的配置项。
type ServerConfig struct {
	Address string `json:"address"`
}

// Default 返回无需配置文件即可在本机安全启动的最小配置。
func Default() Config {
	return Config{
		Server: ServerConfig{
			Address: "127.0.0.1:8080",
		},
	}
}

// Validate 检查服务启动所需的不变量，不负责补默认值。
func (c Config) Validate() error {
	if c.Server.Address == "" {
		return fmt.Errorf("server address is required")
	}

	return nil
}

// Load 在默认配置上应用单个 JSON 文件，并拒绝未知字段和额外顶层值。
// path 为空时直接返回默认配置，保持命令行配置文件为可选项。
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
	// 未知字段通常来自拼写错误；静默忽略会让服务使用非预期的默认值。
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
