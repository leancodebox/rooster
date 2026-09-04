package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/leancodebox/rooster/internal/httpapi"
	"github.com/leancodebox/rooster/internal/roosterapp"
)

func main() {
	if err := run(); err != nil {
		slog.Error("rooster stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	dataDir, err := roosterDataDir()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app, err := roosterapp.Start(ctx, dataDir, listenAddress(), httpapi.WebAssets())
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
	case err := <-app.ServeError():
		if !roosterapp.IsExpectedServerClose(err) {
			return err
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return app.Close(shutdownCtx)
}

func roosterDataDir() (string, error) {
	if value := os.Getenv("ROOSTER_DATA_DIR"); value != "" {
		return value, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config directory: %w", err)
	}
	return filepath.Join(dir, "Rooster"), nil
}
func listenAddress() string {
	if value := os.Getenv("ROOSTER_LISTEN"); value != "" {
		return value
	}
	return "127.0.0.1:9090"
}
