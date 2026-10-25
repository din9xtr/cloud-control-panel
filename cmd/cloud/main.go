package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/din9xtr/cloud-control-panel/internal/app"
	"github.com/din9xtr/cloud-control-panel/internal/command"
	"github.com/din9xtr/cloud-control-panel/internal/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "cloud:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && (args[0] == "cli" || args[0] == "mcp") {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return command.Run(ctx, args[0], args[1:], os.Stdin, os.Stdout, os.Stderr)
	}
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		_, err := fmt.Fprintln(os.Stdout, "Usage: cloud [serve | passwd | healthcheck | cli | mcp]\n  cli  Remote management; run cloud cli help\n  mcp  MCP server over stdio; run cloud mcp --help")
		return err
	}
	if len(args) > 0 && args[0] == "healthcheck" {
		return healthcheck()
	}
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer a.Close()

	command := "serve"
	if len(args) > 0 {
		command, args = args[0], args[1:]
	}
	switch command {
	case "serve":
		if err := a.Bootstrap(ctx); err != nil {
			return err
		}
		return a.Run(ctx)
	case "passwd":
		return setPassword(ctx, a, args)
	default:
		return fmt.Errorf("unknown command %q, expected serve, passwd, healthcheck, cli or mcp", command)
	}
}

func setPassword(ctx context.Context, a *app.App, args []string) error {
	fs := flag.NewFlagSet("passwd", flag.ContinueOnError)
	username := fs.String("username", "admin", "account to update")
	if err := fs.Parse(args); err != nil {
		return err
	}
	fmt.Fprint(os.Stderr, "new password: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return fmt.Errorf("read password: %w", err)
	}
	if err := a.Auth.SetPassword(ctx, *username, strings.TrimRight(line, "\r\n")); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "password updated, all sessions signed out")
	return nil
}

func healthcheck() error {
	_, port, err := net.SplitHostPort(os.Getenv("CLOUD_ADDR"))
	if err != nil {
		port = "8080"
	}
	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return fmt.Errorf("invalid port %q", port)
	}
	target := url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", port), Path: "/healthz"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), http.NoBody)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: %s", resp.Status)
	}
	return nil
}
