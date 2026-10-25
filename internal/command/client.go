package command

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/term"

	"github.com/din9xtr/cloud-control-panel/internal/mcpserver"
	"github.com/din9xtr/cloud-control-panel/internal/remote"
)

func Run(ctx context.Context, mode string, args []string, input io.Reader, output, diagnostic io.Writer) error {
	fs := flag.NewFlagSet("cloud "+mode, flag.ContinueOnError)
	fs.SetOutput(diagnostic)
	serverDefault := os.Getenv("CLOUD_SERVER")
	if serverDefault == "" {
		serverDefault = "http://localhost:8080"
	}
	server := fs.String("server", serverDefault, "HTTP API origin (or CLOUD_SERVER)")
	session := fs.String("session", os.Getenv("CLOUD_SESSION_FILE"), "private session file (or CLOUD_SESSION_FILE)")
	timeout := fs.Duration("timeout", 5*time.Minute, "timeout per HTTP request")
	transferDir := fs.String("transfer-dir", "", "local directory allowed for MCP file transfers")
	readOnly := fs.Bool("read-only", false, "expose only read operations in MCP")
	compact := fs.Bool("json", false, "compact JSON output")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	operations, err := remote.Catalog()
	if err != nil {
		return err
	}
	if mode == "cli" && (fs.NArg() == 0 || fs.Arg(0) == "help") {
		var names []string
		if fs.NArg() > 1 {
			names = fs.Args()[1:]
		}
		return help(output, operations, names)
	}
	if *timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	if mode == "mcp" && fs.NArg() > 0 {
		return errors.New("mcp accepts flags only")
	}
	if *session == "" {
		*session, err = remote.DefaultStateFile(*server)
		if err != nil {
			return err
		}
	}
	client, err := remote.NewClient(*server, *session, *timeout)
	if err != nil {
		return err
	}
	executor := &remote.Executor{Client: client, Restricted: mode == "mcp", Output: output}
	if *transferDir != "" {
		executor.Root, err = os.OpenRoot(*transferDir)
		if err != nil {
			return errors.New("open transfer directory: it must already exist")
		}
		defer executor.Root.Close()
	}
	if mode == "mcp" {
		reader, ok := input.(io.ReadCloser)
		if !ok {
			reader = io.NopCloser(input)
		}
		return mcpserver.New(executor, operations, *readOnly).Run(ctx, &mcp.IOTransport{Reader: reader, Writer: nopWriteCloser{output}})
	}
	executor.Progress = diagnostic
	name := canonical(fs.Arg(0), operations)
	for _, op := range operations {
		if op.Name != name {
			continue
		}
		values, err := arguments(op, fs.Args()[1:], input, diagnostic)
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		if err != nil {
			return err
		}
		result, err := executor.Execute(ctx, op, values)
		if err != nil {
			return err
		}
		destination := output
		if op.Binary && values["output"] == "-" {
			destination = diagnostic
		}
		encoder := json.NewEncoder(destination)
		if !*compact {
			encoder.SetIndent("", "  ")
		}
		return encoder.Encode(result)
	}
	return fmt.Errorf("unknown command %q; run cloud cli help", fs.Arg(0))
}

func arguments(op remote.Operation, args []string, input io.Reader, diagnostic io.Writer) (map[string]any, error) {
	fs := flag.NewFlagSet(op.Name, flag.ContinueOnError)
	fs.SetOutput(diagnostic)
	dataFile := fs.String("data-file", "", "read arguments as JSON from a file; '-' reads stdin")
	properties, _ := op.Schema["properties"].(map[string]any)
	flags := make(map[string]*string, len(properties))
	for name, raw := range properties {
		schema, _ := raw.(map[string]any)
		kind, _ := schema["type"].(string)
		text, _ := schema["description"].(string)
		flags[name] = fs.String(name, "", kind+". "+text)
	}
	fs.Usage = func() {
		_, _ = fmt.Fprintln(diagnostic, op.Description)
		_, _ = fmt.Fprintln(diagnostic, "Usage: cloud cli", op.Name, "[flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, errors.New("unexpected positional arguments; use named flags")
	}
	values := make(map[string]any)
	if *dataFile != "" {
		reader := input
		if *dataFile != "-" {
			f, err := os.Open(*dataFile)
			if err != nil {
				return nil, errors.New("open arguments file")
			}
			defer f.Close()
			reader = f
		}
		data, err := io.ReadAll(io.LimitReader(reader, 2<<20+1))
		if err != nil || len(data) > 2<<20 {
			return nil, errors.New("arguments exceed 2 MiB or could not be read")
		}
		values, err = remote.DecodeArguments(data)
		if err != nil {
			return nil, err
		}
	}
	var parseErr error
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "data-file" || parseErr != nil {
			return
		}
		schema, _ := properties[f.Name].(map[string]any)
		if schema["type"] == "string" {
			values[f.Name] = *flags[f.Name]
			return
		}
		value, err := remote.DecodeArguments([]byte(`{"value":` + *flags[f.Name] + `}`))
		if err != nil {
			parseErr = fmt.Errorf("--%s expects a JSON value", f.Name)
			return
		}
		values[f.Name] = value["value"]
	})
	if parseErr != nil {
		return nil, parseErr
	}
	if (op.Name == "login" || op.Name == "setup") && values["password"] == nil && *dataFile == "" {
		password, err := readPassword(input, diagnostic)
		if err != nil {
			return nil, err
		}
		values["password"] = password
	}
	return values, nil
}

func readPassword(input io.Reader, diagnostic io.Writer) (string, error) {
	if _, err := fmt.Fprint(diagnostic, "Password: "); err != nil {
		return "", fmt.Errorf("prompt for password: %w", err)
	}
	if file, ok := input.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		password, err := term.ReadPassword(int(file.Fd()))
		if _, writeErr := fmt.Fprintln(diagnostic); writeErr != nil {
			return "", fmt.Errorf("finish password prompt: %w", writeErr)
		}
		if err != nil {
			return "", errors.New("read password")
		}
		return string(password), nil
	}
	line, err := bufio.NewReader(io.LimitReader(input, 1024)).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", errors.New("read password")
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func canonical(name string, operations []remote.Operation) string {
	for _, op := range operations {
		for _, alias := range op.Aliases {
			if alias == name {
				return op.Name
			}
		}
	}
	return name
}

func help(output io.Writer, operations []remote.Operation, names []string) error {
	if len(names) > 0 {
		for _, op := range operations {
			if op.Name == canonical(names[0], operations) {
				return json.NewEncoder(output).Encode(op.Schema)
			}
		}
		return errors.New("unknown operation")
	}
	if _, err := fmt.Fprintln(output, "Usage: cloud cli [--server URL] [--session FILE] [--json] COMMAND [flags]\nRun cloud cli COMMAND --help for flags or cloud cli help COMMAND for its JSON schema.\nCanonical commands and aliases come from api/openapi.yaml.\nCommands:"); err != nil {
		return err
	}
	names = make([]string, 0, len(operations))
	for _, op := range operations {
		names = append(names, fmt.Sprintf("  %-22s %s %s", op.Name, op.Method, op.Path))
	}
	sort.Strings(names)
	_, err := fmt.Fprintln(output, strings.Join(names, "\n"))
	return err
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
