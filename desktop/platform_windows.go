package desktop

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func quiet(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
func pickFolder(ctx context.Context) (string, error) {
	c := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-STA", "-Command", `[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; $s=New-Object -ComObject Shell.Application; $f=$s.BrowseForFolder(0,'Choose a Git repository',0,0); if($f){$f.Self.Path}`)
	quiet(c)
	b, e := c.Output()
	return strings.TrimSpace(string(b)), e
}
func OpenWindow(url string) error {
	for _, base := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles")} {
		edge := filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe")
		if _, e := os.Stat(edge); e == nil {
			c := exec.Command(edge, "--app="+url, "--window-size=1440,980")
			quiet(c)
			return c.Start()
		}
	}
	return OpenBrowser(url)
}
func OpenBrowser(url string) error {
	c := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
	quiet(c)
	return c.Start()
}
func ShowError(err error) {
	c := exec.Command("powershell.exe", "-NoProfile", "-Command", `Add-Type -AssemblyName PresentationFramework; [System.Windows.MessageBox]::Show($env:NSF_ERROR,'NotSoFast') | Out-Null`)
	c.Env = append(os.Environ(), "NSF_ERROR="+fmt.Sprint(err))
	quiet(c)
	_ = c.Run()
}

// Cancel the clone and its HTTPS/index-pack helpers before removing staging files.
func cancelTree(c *exec.Cmd) {
	c.Cancel = func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		killer := exec.CommandContext(ctx, filepath.Join(os.Getenv("SystemRoot"), "System32", "taskkill.exe"), "/PID", strconv.Itoa(c.Process.Pid), "/T", "/F")
		quiet(killer)
		if err := killer.Run(); err != nil {
			return c.Process.Kill()
		}
		return nil
	}
	c.WaitDelay = 6 * time.Second
}
