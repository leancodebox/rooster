package roosterSay

import (
	"fyne.io/fyne/v2"
)

var entity fyne.App

func InitFyneApp(app fyne.App) {
	entity = app
}

func Send(msg string) {
	if entity == nil {
		return
	}
	entity.SendNotification(fyne.NewNotification("Rooster", msg))
}
