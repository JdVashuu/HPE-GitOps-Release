package main

import (
	"context"
	"log/slog"
	"os"

	"hpe-recipe/internal/app"
	"hpe-recipe/internal/config"
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Initializing HPE Recipe Detection service...")

	cfg := config.Load()

	application, err := app.New(cfg)
	if err != nil {
		slog.Error("Failed to initialize application", "err", err)
		os.Exit(1)
	}

	if err := application.Run(context.Background()); err != nil {
		slog.Error("Application exited with error", "err", err)
		os.Exit(1)
	}
}
