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
	// ctx 是后续优雅关闭的生命周期入口；当前 Phase 0 尚未接入关闭流程。
	_ = ctx

	logger.Info(
		"starting HTTP server",
		"address", cfg.Server.Address,
	)

	err := server.ListenAndServe()
	// 主动关闭会返回 ErrServerClosed，它属于正常生命周期而不是启动故障。
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return nil
}
