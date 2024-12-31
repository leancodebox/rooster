//go:build darwin || (openbsd && !mips64)

package jobmanager

import (
	"os/exec"
)

func HideWindows(cmd *exec.Cmd) {
}
