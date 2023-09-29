package roosterSay

import (
	"fmt"
	"github.com/gen2brain/beeep"
)

func Send(msg string) {
	err := beeep.Notify("Rooster", msg, "")
	if err != nil {
		fmt.Println(err)
	}
}
