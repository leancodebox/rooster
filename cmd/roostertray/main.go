package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

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
	setupTray(a)

	// Start Server in background
	go runServer(a)

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

func setupTray(a fyne.App) {
	desk, ok := a.(desktop.App)
	if !ok {
		return
	}

	desk.SetSystemTrayIcon(theme.ListIcon())
	// Initial menu state
	updateTrayMenu(desk, []*fyne.MenuItem{
		fyne.NewMenuItem("启动中...", nil),
	})
}

func updateTrayMenu(desk desktop.App, items []*fyne.MenuItem) {
	desk.SetSystemTrayMenu(fyne.NewMenu(appName, items...))
}

func runServer(a fyne.App) {
	serverErr := startRoosterServer()
	port := server.GetPort()
	url := fmt.Sprintf("http://localhost:%d/actor/", port)

	desk, ok := a.(desktop.App)
	if !ok {
		return
	}

	var menuItems []*fyne.MenuItem

	// Open Management Item
	openItem := fyne.NewMenuItem("打开管理", func() {
		if err := openURL(url); err != nil {
			slog.Error("Failed to open URL", "err", err)
		}
	})
	menuItems = append(menuItems, openItem)

	// Port Info
	menuItems = append(menuItems, fyne.NewMenuItem(fmt.Sprintf("端口: %d", port), nil))

	// Error Handling
	if serverErr != nil {
		errItem := fyne.NewMenuItem(fmt.Sprintf("启动错误: %v", serverErr), func() {
			// Allow opening URL even if there's an error reported, similar to original logic
			if err := openURL(url); err != nil {
				slog.Error("Failed to open URL", "err", err)
			}
		})
		menuItems = append(menuItems, errItem)
	} else if port > 0 {
		// Auto open on success
		if err := openURL(url); err != nil {
			slog.Error("Failed to auto-open URL", "err", err)
		}
	}

	// Update menu safely on main thread is usually handled by Fyne,
	// but SetSystemTrayMenu is generally thread-safe or handles it.

	// Add Quit item
	menuItems = append(menuItems, fyne.NewMenuItemSeparator())
	menuItems = append(menuItems, fyne.NewMenuItem("退出", func() {
		a.Quit()
	}))

	updateTrayMenu(desk, menuItems)
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
