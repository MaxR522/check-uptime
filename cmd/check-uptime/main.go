package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/MaxR522/check-uptime/internal/monitor"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("check-uptime stopped", "err", err)
		stop()
		os.Exit(1)
	}
	slog.Info("check-uptime stopped")
}

func run(ctx context.Context) error {
	// In-memory store until the SQLite store exists (roadmap step 4).
	checker := monitor.NewChecker(monitor.NewMemoryStore())
	return checker.RunChecker(ctx)
}
