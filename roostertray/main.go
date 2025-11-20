package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"github.com/leancodebox/rooster/jobmanager"
	"github.com/leancodebox/rooster/jobmanagerserver"
	"github.com/leancodebox/rooster/resource"
)

func logLifecycle(a fyne.App) {
	a.Lifecycle().SetOnStarted(func() {
		slog.Info("Lifecycle: Started")
	})
	a.Lifecycle().SetOnStopped(func() {
		stop()
	})
	a.Lifecycle().SetOnEnteredForeground(func() {
		slog.Info("Lifecycle: Entered Foreground")
	})
	a.Lifecycle().SetOnExitedForeground(func() {
		slog.Info("Lifecycle: Exited Foreground")
	})
}

func main() {
	url := "http://localhost:9090/actor/"
	a := app.New()
	logLifecycle(a)
	a.SetIcon(resource.GetLogo())
	serverErr := startRoosterServer()
	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Error("获取家目录失败", "err", err)
		homeDir = "tmp"
	}
	runLogPath := path.Join(homeDir, ".roosterTaskConfig", "run.log")
	logOut, err := os.OpenFile(runLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		slog.Info("Failed to log to file, using default stderr", "err", err)
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(logOut, &slog.HandlerOptions{
		AddSource: true,
	})))
	// 桌面系统设置托盘
	if desk, ok := a.(desktop.App); ok {
		var list []*fyne.MenuItem
		open := fyne.NewMenuItem("打开管理", func() {
			err := openURL(url)
			if err != nil {
				fmt.Println(err)
			}
		})
		list = append(list, open)
		if serverErr != nil {
			list = append(list, fyne.NewMenuItem(serverErr.Error(), func() {
				err := openURL(url)
				if err != nil {
					fmt.Println(err)
				}
			}))
		} else {
			err := openURL(url)
			if err != nil {
				fmt.Println(err)
			}
		}
		desk.SetSystemTrayIcon(theme.ListIcon())

		m := fyne.NewMenu("rooster-desktop",
			list...,
		)

		desk.SetSystemTrayMenu(m)
	}
	a.Run()
}

func startRoosterServer() error {
	err := jobmanager.RegByUserConfig()
	if err != nil {
		return err
	}
	jobmanagerserver.ServeRun()
	return nil
}

func stop() {
	jobmanagerserver.ServeStop()
}

func openURL(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	default:
		return fmt.Errorf("unsupported platform")
	}
	runCmd := exec.Command(cmd, args...)
	jobmanager.HideWindows(runCmd)
	return runCmd.Start()
}
