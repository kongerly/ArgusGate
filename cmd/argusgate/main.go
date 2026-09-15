package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/kongerly/ArgusGate/internal/app"
	"github.com/kongerly/ArgusGate/internal/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("argusgate exited with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String(
		"config",
		"",
		"Path to the config file",
	)

	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 进程入口只把系统信号转换为 context 取消，应用层不直接依赖操作系统信号。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, cfg); err != nil {
		return fmt.Errorf("run app: %w", err)
	}

	return nil
}
