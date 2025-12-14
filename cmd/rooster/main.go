package main

import (
	_ "embed"
	"log/slog"
	"os"
	"os/signal"

	"github.com/leancodebox/rooster/internal/jobmanager"
	"github.com/leancodebox/rooster/internal/server"
)

func init() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
}

func main() {
	err := jobmanager.RegByUserConfig()
	if err != nil {
		slog.Error(err.Error())
		return
	}
	server.ServeRun()
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	server.ServeStop()
	slog.Info("bye~~👋👋")
}
