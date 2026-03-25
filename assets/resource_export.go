package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed logo.png
var Logo []byte

//go:embed trayicon.png
var trayIcon []byte

func GetLogo() fyne.Resource {
	return &fyne.StaticResource{
		StaticName:    "logo.png",
		StaticContent: Logo,
	}
}

func GetTrayIcon() fyne.Resource {
	return &fyne.StaticResource{
		StaticName:    "trayicon.png",
		StaticContent: trayIcon,
	}
}
