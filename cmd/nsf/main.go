package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"notsofast/core"
	"notsofast/protocol"
)

func main() {
	if e := run(); e != nil {
		json.NewEncoder(os.Stderr).Encode(map[string]string{"reason": core.Code(e), "message": e.Error()})
		os.Exit(1)
	}
}
func run() error {
	config := flag.String("config", "config.json", "operator configuration file")
	user := flag.String("user", "local", "trusted local CLI principal")
	expand := flag.Bool("expand", false, "include full manifest/evaluations")
	listen := flag.String("listen", "127.0.0.1:8787", "HTTP listen address")
	flag.Parse()
	if flag.NArg() != 1 {
		return fmt.Errorf("usage: nsf -config config.json -user local [-expand] <operation> < request.json")
	}
	b, e := os.ReadFile(*config)
	if e != nil {
		return e
	}
	var cfg core.Config
	if e = json.Unmarshal(b, &cfg); e != nil {
		return e
	}
	if token := os.Getenv("NSF_TOKEN"); token != "" {
		if cfg.Tokens == nil {
			cfg.Tokens = map[string]string{}
		}
		cfg.Tokens[token] = *user
	}
	s, e := core.Open(cfg)
	if e != nil {
		return e
	}
	defer s.Close()
	if flag.Arg(0) == "serve" {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
		server := http.Server{Addr: *listen, Handler: protocol.NewHandler(s, cfg.Workers), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 65 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
		return server.ListenAndServe()
	}
	body, e := io.ReadAll(io.LimitReader(os.Stdin, (2<<20)+1))
	if e != nil {
		return e
	}
	if len(body) > 2<<20 {
		return fmt.Errorf("request too large")
	}
	if len(body) == 0 {
		body = []byte("{}")
	}
	result, e := protocol.Dispatch(context.Background(), s, *user, flag.Arg(0), body, *expand)
	if e != nil {
		return e
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
