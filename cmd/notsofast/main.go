// NotSoFast opens the embedded local operator workspace without a terminal.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"notsofast/desktop"
)

func main() {
	if err := run(); err != nil {
		desktop.ShowError(err)
		os.Exit(1)
	}
}
func run() error {
	data, _ := os.UserConfigDir()
	root := flag.String("data", filepath.Join(data, "NotSoFast"), "application data directory")
	headless := flag.Bool("no-open", false, "serve without opening a window")
	flag.Parse()
	// Prefer the bundled, versioned Git distribution if present.
	exe, _ := os.Executable()
	bundled := filepath.Join(filepath.Dir(exe), "git", "cmd")
	if _, err := os.Stat(filepath.Join(bundled, "git.exe")); err == nil {
		os.Setenv("PATH", bundled+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	sessionFile := filepath.Join(*root, "session.json")
	var old struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if b, err := os.ReadFile(sessionFile); err == nil && json.Unmarshal(b, &old) == nil {
		// Only reuse our loopback session; a corrupted file cannot redirect a launcher to a remote URL.
		parsed, parseErr := url.Parse(old.URL)
		if parseErr == nil && parsed.Scheme == "http" && parsed.Hostname() == "127.0.0.1" && parsed.Port() != "" && parsed.User == nil && parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment == "" {
			req, err := http.NewRequest("POST", old.URL+"/api/ping", nil)
			if err == nil {
				req.Header.Set("X-NSF-Session", old.Token)
				client := http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
				if res, err := client.Do(req); err == nil {
					res.Body.Close()
					if res.StatusCode == 200 {
						if !*headless {
							return desktop.OpenWindow(old.URL + "/#" + old.Token)
						}
						return fmt.Errorf("NotSoFast is already running")
					}
				}
			}
		}
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := hex.EncodeToString(raw)
	app, err := desktop.New(*root, token)
	if err != nil {
		return err
	}
	defer app.Close()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	app.Host = listener.Addr().String()
	url := "http://" + app.Host
	server := &http.Server{Handler: app, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 65 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	stopping := make(chan struct{}, 1)
	app.Quit = func() {
		select {
		case stopping <- struct{}{}:
		default:
		}
	}
	b, _ := json.Marshal(map[string]string{"url": url, "token": token})
	if err = os.WriteFile(sessionFile, b, 0600); err != nil {
		listener.Close()
		return err
	}
	defer os.Remove(sessionFile)
	go func() { _ = server.Serve(listener) }()
	if *headless {
		fmt.Println(url)
	} else if err = desktop.OpenWindow(url + "/#" + token); err != nil {
		server.Close()
		return err
	}
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	defer signal.Stop(signalCh)
	select {
	case <-stopping:
	case <-signalCh:
	}
	ctx, cancel := context.WithTimeout(context.Background(), 65*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}
