package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/kongerly/ArgusGate/internal/config"
	"github.com/kongerly/ArgusGate/internal/httpapi"
	"github.com/kongerly/ArgusGate/internal/proxy"
)

const chatCompletionsPath = "/v1/chat/completions"

// Run 是应用的 composition root，负责组装依赖并持有 HTTP Server 生命周期。
func Run(ctx context.Context, cfg config.Config) error {
	logger := slog.New(
		slog.NewTextHandler(os.Stdout, nil),
	)

	shutdownTimeout, err := time.ParseDuration(cfg.Server.ShutdownTimeout)
	if err != nil {
		return fmt.Errorf("parse shutdown timeout: %w", err)
	}

	endpoint, err := buildBackendEndpoint(cfg.Backends[0].Origin)
	if err != nil {
		return fmt.Errorf("build backend endpoint: %w", err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	p := proxy.New(client, endpoint, cfg.Backends[0].Authorization)

	handler := httpapi.NewHandler(logger, p)

	server := &http.Server{
		Addr:    cfg.Server.Address,
		Handler: handler,
	}

	logger.Info(
		"starting HTTP server",
		"address", cfg.Server.Address,
	)

	errCh := make(chan error, 1)

	go func() {
		err := server.ListenAndServe()
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)

	case <-ctx.Done():
		// 关闭宽限期使用独立 context，避免触发关闭的父 context 立即取消 Shutdown。
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			shutdownTimeout,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
		return nil
	}
}

// buildBackendEndpoint 把经过校验的 backend origin 和固定的
// chat completions path 组合成完整的 upstream URL。
// 它假设 origin 已经由 config.Validate 校验通过，
// 但仍然处理 url.Parse 自身可能返回的错误。
func buildBackendEndpoint(origin string) (*url.URL, error) {
	u, err := url.Parse(origin)
	if err != nil {
		return nil, fmt.Errorf("parse backend origin %q: %w", origin, err)
	}

	u.Path = chatCompletionsPath

	return u, nil
}
