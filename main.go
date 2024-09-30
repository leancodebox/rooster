package main

import (
	_ "embed"
	"github.com/leancodebox/rooster/jobmanager"
	"github.com/leancodebox/rooster/jobmanagerserver"
	"log/slog"
	"os"
	"os/signal"
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
	jobmanagerserver.ServeRun()
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	jobmanagerserver.ServeStop()
	slog.Info("bye~~👋👋")
}
