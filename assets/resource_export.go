package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed trayicon.png
var trayIcon []byte

func GetTrayIcon() fyne.Resource {
	return &fyne.StaticResource{
		StaticName:    "trayicon.png",
		StaticContent: trayIcon,
	}
}
