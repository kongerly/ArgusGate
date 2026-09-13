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
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}

		return nil
	}

}
