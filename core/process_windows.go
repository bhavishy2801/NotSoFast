package core

import (
	"os/exec"
	"syscall"
)

func quietProcess(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
