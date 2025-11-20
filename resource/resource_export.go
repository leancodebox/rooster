package resource

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed logo.png
var Logo []byte

func GetLogo() fyne.Resource {
	return &fyne.StaticResource{
		StaticName:    "logo.png",
		StaticContent: Logo,
	}
}
