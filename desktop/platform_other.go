//go:build !windows

package desktop

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

func quiet(c *exec.Cmd) {}
func pickFolder(ctx context.Context) (string, error) {
	return "", fmt.Errorf("paste the repository folder path on this platform")
}
func OpenWindow(url string) error {
	command := "xdg-open"
	if runtime.GOOS == "darwin" {
		command = "open"
	}
	return exec.Command(command, url).Start()
}
func OpenBrowser(url string) error { return OpenWindow(url) }
func ShowError(err error)          { fmt.Fprintln(os.Stderr, err) }

func cancelTree(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error { return syscall.Kill(-c.Process.Pid, syscall.SIGKILL) }
	c.WaitDelay = 6 * time.Second
}
