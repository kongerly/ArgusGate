package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/kongerly/ArgusGate/internal/app"
	"github.com/kongerly/ArgusGate/internal/config"
)

func main() {
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

	fmt.Println("ArgusGate is running...")
	fmt.Println(cfg.Server.Address)

	ctx := context.Background()

	if err := app.Run(ctx, cfg); err != nil {
		fmt.Println(err)
	}
}
