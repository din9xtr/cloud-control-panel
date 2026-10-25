package remote

import (
	"fmt"
	"sort"
	"strings"
)

func CommandReference() (string, error) {
	operations, err := Catalog()
	if err != nil {
		return "", err
	}
	var out strings.Builder
	out.WriteString("# Command and tool reference\n\nGenerated from [api/openapi.yaml](../api/openapi.yaml). Do not edit this table by hand.\nRegenerate with `go run ./cmd/cloud-docs`; verify with `go run ./cmd/cloud-docs --check`.\n\nCanonical names are both CLI commands and MCP tools. Aliases are CLI-only.\nHTTP operations use `/api/v1` as their prefix. Local transfer helpers call the HTTP API.\nAll arguments are named CLI flags or flat MCP arguments.\n\n| Command / MCP tool | CLI aliases | Required arguments | Optional arguments | Operation |\n| --- | --- | --- | --- | --- |\n")
	for _, op := range operations {
		var required, optional []string
		requiredSet := make(map[string]bool)
		for _, name := range requiredStrings(op.Schema["required"]) {
			requiredSet[name] = true
		}
		for name := range object(op.Schema["properties"]) {
			if requiredSet[name] {
				required = append(required, "`"+name+"`")
			} else {
				optional = append(optional, "`"+name+"`")
			}
		}
		sort.Strings(required)
		sort.Strings(optional)
		operation := "Local transfer helper"
		if op.Path != "" {
			operation = "`" + op.Method + " " + op.Path + "`"
		}
		fmt.Fprintf(&out, "| `%s` | %s | %s | %s | %s |\n", op.Name, referenceList(op.Aliases), referenceList(required), referenceList(optional), operation)
	}
	out.WriteString("\nUse `cloud cli help COMMAND` for its complete JSON Schema, `cloud cli COMMAND --help` for flags,\nor MCP tool discovery for the same schema. See [connection setup](clients.md), [CLI examples](cli.md)\nand [MCP configuration](mcp.md).\n")
	return out.String(), nil
}

func requiredStrings(value any) []string {
	if names, ok := value.([]string); ok {
		return names
	}
	var names []string
	for _, raw := range array(value) {
		names = append(names, stringValue(raw))
	}
	return names
}

func referenceList(values []string) string {
	if len(values) == 0 {
		return "—"
	}
	return strings.Join(values, ", ")
}
