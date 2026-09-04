package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/leancodebox/rooster/internal/httpapi"
	"github.com/leancodebox/rooster/internal/roosterapp"
)

const trayIcon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect width="64" height="64" rx="14" fill="#ea6823"/><path d="M12 33h10l6-17 9 34 7-20h8" fill="none" stroke="#fff" stroke-width="5" stroke-linecap="round" stroke-linejoin="round"/><circle cx="49" cy="15" r="4" fill="#6ee7c5"/></svg>`

func main() {
	dataDir, err := roosterDataDir()
	if err != nil {
		slog.Error("resolve data directory", "error", err)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	core, err := roosterapp.Start(ctx, dataDir, listenAddress(), httpapi.WebAssets())
	if err != nil {
		slog.Error("start rooster", "error", err)
		return
	}
	desktopApp := app.NewWithID("com.leancodebox.rooster")
	icon := fyne.NewStaticResource("rooster.svg", []byte(trayIcon))
	desktopApp.SetIcon(icon)
	var closeOnce sync.Once
	closeCore := func() {
		closeOnce.Do(func() {
			cancel()
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			if err := core.Close(shutdownCtx); err != nil {
				slog.Error("stop rooster", "error", err)
			}
		})
	}
	desktopApp.Lifecycle().SetOnStopped(closeCore)
	if tray, ok := desktopApp.(desktop.App); ok {
		tray.SetSystemTrayIcon(icon)
		tray.SetSystemTrayMenu(fyne.NewMenu("Rooster", fyne.NewMenuItem("打开控制台", func() { openBrowser(core.URL()) }), fyne.NewMenuItem(fmt.Sprintf("监听 %s", core.URL()), nil), fyne.NewMenuItemSeparator(), fyne.NewMenuItem("退出", func() { desktopApp.Quit() })))
	}
	openBrowser(core.URL())
	desktopApp.Run()
	closeCore()
}

func openBrowser(url string) {
	go func() {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", "", url)
		case "darwin":
			cmd = exec.Command("open", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}
		if err := cmd.Run(); err != nil {
			slog.Error("open browser", "error", err)
		}
	}()
}
func roosterDataDir() (string, error) {
	if value := os.Getenv("ROOSTER_DATA_DIR"); value != "" {
		return value, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Rooster"), nil
}
func listenAddress() string {
	if value := os.Getenv("ROOSTER_LISTEN"); value != "" {
		return value
	}
	return "127.0.0.1:9090"
}
