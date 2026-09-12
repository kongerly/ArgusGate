package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/kongerly/ArgusGate/internal/config"
	"github.com/kongerly/ArgusGate/internal/httpapi"
)

func Run(ctx context.Context, cfg config.Config) error {
	logger := slog.New(
		slog.NewTextHandler(os.Stdout, nil),
	)

	handler := httpapi.NewHandler(logger)

	server := &http.Server{
		Addr:    cfg.Server.Address,
		Handler: handler,
	}
	_ = ctx

	logger.Info(
		"starting HTTP server",
		"address", cfg.Server.Address,
	)

	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return nil
}
