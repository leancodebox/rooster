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
	"github.com/leancodebox/rooster/assets"
	"github.com/leancodebox/rooster/jobmanager"
	"github.com/leancodebox/rooster/jobmanagerserver"
)

func logLifecycle(a fyne.App) {
	a.Lifecycle().SetOnStarted(func() {
		slog.Info("Lifecycle: Started")
	})
	a.Lifecycle().SetOnStopped(func() {
		slog.Info("Lifecycle: Stop")
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
	roosterApp := app.NewWithID("com.leancodebox.rooster")
	logLifecycle(roosterApp)
	roosterApp.SetIcon(assets.GetAppIcon())
	// 桌面系统设置托盘
	if desk, ok := roosterApp.(desktop.App); ok {
		desk.SetSystemTrayIcon(theme.ListIcon())
		m := fyne.NewMenu("rooster-desktop启动中")
		desk.SetSystemTrayMenu(m)
	}
	go func() {
		serverErr := startRoosterServer()
		port := jobmanagerserver.GetPort()
		url := fmt.Sprintf("http://localhost:%d/actor/", port)
		// 桌面系统设置托盘
		if desk, ok := roosterApp.(desktop.App); ok {
			var list []*fyne.MenuItem
			open := fyne.NewMenuItem("打开管理", func() {
				err := openURL(url)
				if err != nil {
					slog.Info(err.Error())
				}
			})
			list = append(list, open)
			list = append(list, fyne.NewMenuItem(fmt.Sprintf("端口: %d", port), nil))
			if serverErr != nil {
				list = append(list, fyne.NewMenuItem(serverErr.Error(), func() {
					err := openURL(url)
					if err != nil {
						slog.Info(err.Error())
					}
				}))
			} else {
				if port > 0 {
					err := openURL(url)
					if err != nil {
						slog.Info(err.Error())
					}
				}
			}

			m := fyne.NewMenu("rooster-desktop",
				list...,
			)
			desk.SetSystemTrayMenu(m)
		}
	}()
	//roosterSay.InitFyneApp(roosterApp)
	roosterApp.Run()
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
