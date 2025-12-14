package jobmanager

import (
	"fmt"
	"github.com/robfig/cron/v3"
	"testing"
	"time"
)

func TestAddFunc(t *testing.T) {
	var c = cron.New(cron.WithSeconds())
	go c.Run()

	_, err := c.AddFunc("* * * * * *", func() {
		fmt.Println("hello")
	})
	if err != nil {
		fmt.Println(err)
	}

	c.AddFunc("* * * * * *", func() {
		fmt.Println("hello")
	})

	time.Sleep(5 * time.Second)
}
