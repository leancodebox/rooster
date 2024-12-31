package jobmanager

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestName(t *testing.T) {
	//ctx := context.Background()
	//cmd := exec.CommandContext(ctx,"echo","1212")

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		select {
		case <-ctx.Done():
			t.Log("done")
		}
	}()

	t.Log("waitCancel")
	cancel()
	t.Log("123454321")
	time.Sleep(time.Second * 3)
}

func TestSH(t *testing.T) {

	sendFromAdmin := make(chan bool)
	defer func() {
		close(sendFromAdmin)
	}()
	cmdIsLife := true
	// 外层的 gochannel 负责重启
	go func() {
		sendToParent := make(chan bool)
		defer func() {
			cmdIsLife = false
			close(sendToParent)
		}()
		consecutiveFailures := 0
		for {
			// build start
			unitStartTime := time.Now()
			ctx, cancel := context.WithCancel(context.Background())
			cmd := exec.CommandContext(ctx, "/opt/homebrew/bin/php", "/Users/thh/workspace/about/tmp/p.php")
			cmd.Stdout = os.Stdout
			// build end
			go func() {
				defer func() {
					if err := recover(); err != nil {
						t.Log("panic", err)
					}
					sendToParent <- true
				}()
				// 准备启动
				if err := cmd.Start(); err != nil {
					slog.Error("cmd.start", "err", err.Error())
				}
				if err := cmd.Wait(); err != nil {
					slog.Error("cmd.start", "err", err.Error())
				}
			}()
			observe := false
			for {
				select {
				case <-sendFromAdmin:
					cancel()
				case <-sendToParent:
					observe = true
				}
				if observe {
					break
				}
			}
			executionTime := time.Since(unitStartTime)
			if executionTime <= maxExecutionTime {
				consecutiveFailures += 1
			} else {
				consecutiveFailures = 0
			}

			if consecutiveFailures >= maxConsecutiveFailures {
				msg := "程序连续3次启动失败，停止重启"
				slog.Info(msg)
				break
			} else {
				msg := "程序终止尝试重新运行"
				slog.Info(msg)
			}
		}

	}()
	slog.Info("adsa")

	time.Sleep(time.Second * 1)
	if cmdIsLife {
		sendFromAdmin <- true
	}
	time.Sleep(time.Second * 2)

	if cmdIsLife {
		sendFromAdmin <- true
	}
	time.Sleep(time.Second * 4)
}
