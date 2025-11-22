package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed app.png
var appPng []byte

func GetAppIcon() fyne.Resource {
	return &fyne.StaticResource{StaticName: "app.png", StaticContent: appPng}
}
