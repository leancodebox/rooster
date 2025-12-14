package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"github.com/leancodebox/rooster/assets"
	"github.com/leancodebox/rooster/internal/jobmanager"
	"github.com/leancodebox/rooster/internal/server"
)

const (
	appID   = "com.leancodebox.rooster"
	appName = "rooster-desktop"
)

func main() {
	setupLogger()

	a := app.NewWithID(appID)
	a.SetIcon(assets.GetAppIcon())

	setupLifecycle(a)

	// Initialize Tray with loading state
	menu := setupTray(a)

	// Start Server in background
	go runServer(a, menu)

	a.Run()
}

func setupLogger() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Error("Failed to get home directory", "err", err)
		homeDir = "tmp"
	}

	logDir := filepath.Join(homeDir, ".roosterTaskConfig")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		slog.Error("Failed to create log directory", "err", err)
	}

	runLogPath := filepath.Join(logDir, "run.log")
	logOut, err := os.OpenFile(runLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		slog.Info("Failed to log to file, using default stderr", "err", err)
		return
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(logOut, &slog.HandlerOptions{
		AddSource: true,
	})))
}

func setupLifecycle(a fyne.App) {
	lifecycle := a.Lifecycle()
	lifecycle.SetOnStarted(func() {
		slog.Info("Lifecycle: Started")
	})
	lifecycle.SetOnStopped(func() {
		slog.Info("Lifecycle: Stop")
		server.ServeStop()
	})
	lifecycle.SetOnEnteredForeground(func() {
		slog.Info("Lifecycle: Entered Foreground")
	})
	lifecycle.SetOnExitedForeground(func() {
		slog.Info("Lifecycle: Exited Foreground")
	})
}

func setupTray(a fyne.App) *fyne.Menu {
	desk, ok := a.(desktop.App)
	if !ok {
		return nil
	}

	desk.SetSystemTrayIcon(theme.ListIcon())
	// Initial menu state
	menu := fyne.NewMenu("启动中", fyne.NewMenuItem("启动中...", nil))
	desk.SetSystemTrayMenu(menu)
	return menu
}

func runServer(a fyne.App, menu *fyne.Menu) {
	serverErr := startRoosterServer()
	port := server.GetPort()
	url := fmt.Sprintf("http://localhost:%d/actor/", port)

	// Rebuild the menu items from scratch
	var newMenuItems []*fyne.MenuItem

	// Open Management Item
	openItem := fyne.NewMenuItem("打开管理", func() {
		if err := openURL(url); err != nil {
			slog.Error("Failed to open URL", "err", err)
		}
	})
	newMenuItems = append(newMenuItems, openItem)

	// Port Info
	newMenuItems = append(newMenuItems, fyne.NewMenuItem(fmt.Sprintf("端口: %d", port), nil))

	// Error Handling
	if serverErr != nil {
		errItem := fyne.NewMenuItem(fmt.Sprintf("启动错误: %v", serverErr), func() {
			// Allow opening URL even if there's an error reported, similar to original logic
			if err := openURL(url); err != nil {
				slog.Error("Failed to open URL", "err", err)
			}
		})
		newMenuItems = append(newMenuItems, errItem)
	} else if port > 0 {
		// Auto open on success
		if err := openURL(url); err != nil {
			slog.Error("Failed to auto-open URL", "err", err)
		}
	}

	// NOTE: Fyne usually appends a "Quit" item automatically in systray menus on some platforms.
	// If it doesn't, or if we want an explicit one, we can add it.
	// Based on user feedback, it seems items were duplicated/appended.
	// To avoid duplicates, we rely on Fyne's default behavior for Quit if present,
	// OR we assume that by refreshing the SAME menu object, we avoid the append issue.
	// Let's adding Quit explicitly but safely.
	newMenuItems = append(newMenuItems, fyne.NewMenuItemSeparator())
	newMenuItems = append(newMenuItems, fyne.NewMenuItem("退出", func() {
		a.Quit()
	}))
	time.Sleep(1000 * time.Millisecond)
	if desk, ok := a.(desktop.App); ok {
		desk.SetSystemTrayMenu(fyne.NewMenu(appName, newMenuItems...))
	}
}

func startRoosterServer() error {
	if err := jobmanager.RegByUserConfig(); err != nil {
		return err
	}
	server.ServeRun()
	return nil
}

func openURL(url string) error {
	var cmdName string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmdName = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmdName = "open"
		args = []string{url}
	case "linux":
		cmdName = "xdg-open"
		args = []string{url}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	runCmd := exec.Command(cmdName, args...)
	jobmanager.HideWindows(runCmd)
	return runCmd.Start()
}
