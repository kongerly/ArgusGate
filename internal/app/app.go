package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/kongerly/ArgusGate/internal/config"
	"github.com/kongerly/ArgusGate/internal/httpapi"
)

// Run 是应用的 composition root，负责组装依赖并持有 HTTP Server 生命周期。
func Run(ctx context.Context, cfg config.Config) error {
	logger := slog.New(
		slog.NewTextHandler(os.Stdout, nil),
	)

	shutdownTimeout, err := time.ParseDuration(cfg.Server.ShutdownTimeout)
	if err != nil {
		return fmt.Errorf("parse shutdown timeout: %w", err)
	}

	handler := httpapi.NewHandler(logger)

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
