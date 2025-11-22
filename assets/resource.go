package assets

import (
    _ "embed"
    "fyne.io/fyne/v2"
)

//go:embed app.png
var appPng []byte

//go:embed tray.png
var trayPng []byte

func GetAppIcon() fyne.Resource {
    return &fyne.StaticResource{StaticName: "app.png", StaticContent: appPng}
}

func GetTrayIcon() fyne.Resource {
    return &fyne.StaticResource{StaticName: "tray.png", StaticContent: trayPng}
}

