//go:build !windows

package desktop

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
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
func ShowError(err error) { fmt.Fprintln(os.Stderr, err) }
