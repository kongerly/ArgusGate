package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kongerly/ArgusGate/internal/app"
	"github.com/kongerly/ArgusGate/internal/config"
)

func main() {
	// 进程入口只处理启动参数和最终错误；服务组装与运行细节集中在 app 包。
	configPath := flag.String(
		"config",
		"",
		"Path to the config file",
	)

	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Println(err)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 内部包返回错误而不直接退出进程，退出策略由 main 统一决定。
	if err := app.Run(ctx, cfg); err != nil {
		fmt.Println(err)
	}
}
