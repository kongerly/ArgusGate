package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config 汇总启动时读取的静态配置；加载完成后调用方应将其视为不可变值。
type Config struct {
	Server   ServerConfig    `json:"server"`
	Backends []BackendConfig `json:"backends"`
}

// ServerConfig 定义 HTTP Server 当前阶段实际使用的配置项。
type ServerConfig struct {
	Address         string `json:"address"`
	ShutdownTimeout string `json:"shutdown_timeout"`
}

// BackendConfig 描述 Phase 1 使用的单个静态上游。
type BackendConfig struct {
	// Origin 是 backend 的 HTTP(S) origin，不包含业务路径、query 或凭据。
	Origin string `json:"origin"`
	// Authorization 是可选的完整上游 Authorization header 值；当前按原值发送。
	// 配置文件中的真实凭据不得提交到版本库。
	Authorization string `json:"authorization"`
}

// Default 返回无需配置文件即可在本机安全启动的最小配置。
func Default() Config {
	return Config{
		Server: ServerConfig{
			Address:         "127.0.0.1:8080",
			ShutdownTimeout: "5s",
		},
		Backends: []BackendConfig{
			{
				Origin:        "http://127.0.0.1:8081",
				Authorization: "",
			},
		},
	}
}

// Validate 检查服务启动所需的不变量，不负责补默认值。
func (c Config) Validate() error {
	if strings.TrimSpace(c.Server.Address) == "" {
		return fmt.Errorf("server.address must not be empty")
	}

	if strings.TrimSpace(c.Server.ShutdownTimeout) == "" {
		return fmt.Errorf("server.shutdown_timeout must not be empty")
	}

	if _, err := time.ParseDuration(c.Server.ShutdownTimeout); err != nil {
		return fmt.Errorf("server.shutdown_timeout must be a valid duration: %w", err)
	}

	// Phase 1 通过单 backend 直连建立代理闭环；当前不支持多 backend 选择。
	if len(c.Backends) != 1 {
		return fmt.Errorf("exactly one backend is required, got %d", len(c.Backends))
	}

	if err := validateBackendOrigin(c.Backends[0].Origin); err != nil {
		return fmt.Errorf("validate backend origin: %w", err)
	}

	return nil
}

// validateBackendOrigin 检查 backend.origin 是否符合 P1 的 URL 规则。
// 它只描述"后端在哪"，不携带 credential、path、query 等业务信息。
func validateBackendOrigin(origin string) error {
	if strings.TrimSpace(origin) == "" {
		return fmt.Errorf("must not be empty")
	}

	u, err := url.Parse(origin)
	if err != nil {
		return fmt.Errorf("%q is not a valid URL: %w", origin, err)
	}

	if !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("scheme must be http or https, got %q", u.Scheme)
	}

	if u.Hostname() == "" {
		return fmt.Errorf("host must not be empty")
	}

	if u.User != nil {
		return fmt.Errorf("userinfo must not be present, use authorization field instead")
	}

	if u.Path != "" && u.Path != "/" {
		return fmt.Errorf("path must be empty or %q, got %q", "/", u.Path)
	}

	if u.RawQuery != "" || u.ForceQuery {
		return fmt.Errorf("query must not be present")
	}

	if u.Fragment != "" {
		return fmt.Errorf("fragment must not be present, got %q", u.Fragment)
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
