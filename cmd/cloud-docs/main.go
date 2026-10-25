package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/din9xtr/cloud-control-panel/internal/remote"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	check := flag.Bool("check", false, "fail if the generated reference is stale")
	output := flag.String("output", "docs/commands.md", "reference file")
	flag.Parse()
	text, err := remote.CommandReference()
	if err != nil {
		return err
	}
	if *check {
		data, err := os.ReadFile(*output)
		if err != nil {
			return err
		}
		if string(data) != text {
			return errors.New("command reference is stale; run go run ./cmd/cloud-docs")
		}
		return nil
	}
	return os.WriteFile(*output, []byte(text), 0o600)
}
