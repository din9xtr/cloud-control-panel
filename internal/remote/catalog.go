package remote

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/pb33f/go-yaml"

	"github.com/din9xtr/cloud-control-panel/api"
)

type Parameter struct {
	Name string
	In   string
}

type Operation struct {
	Aliases     []string
	Name        string
	Method      string
	Path        string
	Description string
	Parameters  []Parameter
	BodyFields  []string
	ContentType string
	Binary      bool
	Tus         bool
	Schema      map[string]any
	validator   *jsonschema.Resolved
}

func Catalog() ([]Operation, error) {
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(api.Contract()), &doc); err != nil {
		return nil, fmt.Errorf("read API contract: %w", err)
	}
	paths := object(doc["paths"])
	var operations []Operation
	for route, raw := range paths {
		item := object(raw)
		for _, method := range []string{"get", "post", "put", "delete", "head", "options", "patch"} {
			definition := object(item[method])
			if len(definition) == 0 {
				continue
			}
			op, err := parseOperation(doc, route, method, item, definition)
			if err != nil {
				return nil, err
			}
			operations = append(operations, op)
		}
	}
	for _, raw := range array(doc["x-client-operations"]) {
		definition := object(raw)
		schema, err := expand(doc, definition["inputSchema"], 0)
		if err != nil {
			return nil, err
		}
		op := Operation{Name: stringValue(definition["operationId"]), Method: stringValue(definition["method"]), Description: stringValue(definition["description"]), Schema: object(schema), Aliases: operationAliases(definition)}
		if op.Name == "" || op.Method == "" || len(op.Schema) == 0 {
			return nil, errors.New("invalid client operation in API contract")
		}
		operations = append(operations, op)
	}
	seen := make(map[string]bool, len(operations))
	for i := range operations {
		if seen[operations[i].Name] {
			return nil, fmt.Errorf("duplicate operationId: %s", operations[i].Name)
		}
		seen[operations[i].Name] = true
		encoded, err := json.Marshal(operations[i].Schema)
		if err != nil {
			return nil, fmt.Errorf("encode operation schema: %w", err)
		}
		var schema jsonschema.Schema
		if err := json.Unmarshal(encoded, &schema); err != nil {
			return nil, fmt.Errorf("decode operation schema: %w", err)
		}
		resolved, err := schema.Resolve(nil)
		if err != nil {
			return nil, fmt.Errorf("resolve %s schema: %w", operations[i].Name, err)
		}
		operations[i].validator = resolved
	}
	for _, op := range operations {
		for _, alias := range op.Aliases {
			if alias == "" || seen[alias] {
				return nil, fmt.Errorf("duplicate or empty command alias: %s", alias)
			}
			seen[alias] = true
		}
	}
	sort.Slice(operations, func(i, j int) bool { return operations[i].Name < operations[j].Name })
	return operations, nil
}

func parseOperation(doc map[string]any, route, method string, item, definition map[string]any) (Operation, error) {
	op := Operation{Name: stringValue(definition["operationId"]), Method: strings.ToUpper(method), Path: route, Aliases: operationAliases(definition)}
	if op.Name == "" {
		return op, fmt.Errorf("missing operationId: %s %s", method, route)
	}
	op.Description = stringValue(definition["description"])
	if op.Description == "" {
		op.Description = op.Name + " (" + op.Method + " " + route + ")"
	}
	properties := make(map[string]any)
	var required []string
	parameters := append([]any(nil), array(item["parameters"])...)
	parameters = append(parameters, array(definition["parameters"])...)
	for _, raw := range parameters {
		parameter, err := expand(doc, raw, 0)
		if err != nil {
			return op, err
		}
		p := object(parameter)
		name := stringValue(p["name"])
		if name == "Tus-Resumable" {
			op.Tus = true
			continue
		}
		schema, err := expand(doc, p["schema"], 0)
		if err != nil {
			return op, err
		}
		properties[name] = schema
		op.Parameters = append(op.Parameters, Parameter{Name: name, In: stringValue(p["in"])})
		if p["required"] == true {
			required = append(required, name)
		}
	}
	expandedBody, err := expand(doc, definition["requestBody"], 0)
	if err != nil {
		return op, err
	}
	content := object(object(expandedBody)["content"])
	if len(content) > 1 {
		return op, fmt.Errorf("%s requires an explicit client adapter for multiple request media types", op.Name)
	}
	for contentType, raw := range content {
		op.ContentType = contentType
		if contentType == "application/json" {
			expanded, err := expand(doc, object(raw)["schema"], 0)
			if err != nil {
				return op, err
			}
			body := object(expanded)
			if body["type"] != "object" {
				return op, fmt.Errorf("%s requires an explicit client adapter for a non-object JSON body", op.Name)
			}
			for name, schema := range object(body["properties"]) {
				if _, exists := properties[name]; exists {
					return op, fmt.Errorf("%s has colliding body and parameter names", op.Name)
				}
				properties[name] = schema
				op.BodyFields = append(op.BodyFields, name)
			}
			for _, name := range array(body["required"]) {
				required = append(required, stringValue(name))
			}
		} else {
			properties["data"] = map[string]any{"type": "string", "maxLength": 1398104, "description": "Base64-encoded chunk, at most 1 MiB decoded."}
			required = append(required, "data")
		}
	}
	for status, raw := range object(definition["responses"]) {
		if !strings.HasPrefix(status, "2") {
			continue
		}
		expanded, err := expand(doc, raw, 0)
		if err != nil {
			return op, err
		}
		for contentType := range object(object(expanded)["content"]) {
			if contentType != "application/json" {
				op.Binary = true
			}
		}
	}
	if op.Binary {
		properties["output"] = map[string]any{"type": "string", "minLength": 1, "description": "Local destination. Relative to transfer-dir in MCP; '-' streams to stdout in CLI."}
		required = append(required, "output")
	}
	op.Schema = map[string]any{"type": "object", "additionalProperties": false, "properties": properties, "required": required}
	if required == nil {
		op.Schema["required"] = []string{}
	}
	return op, nil
}

func expand(doc map[string]any, value any, depth int) (any, error) {
	if depth > 64 {
		return nil, fmt.Errorf("API reference depth exceeded")
	}
	switch v := value.(type) {
	case map[string]any:
		if ref := stringValue(v["$ref"]); ref != "" {
			if !strings.HasPrefix(ref, "#/") {
				return nil, fmt.Errorf("unsupported API reference")
			}
			var target any = doc
			for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
				target = object(target)[part]
			}
			if target == nil {
				return nil, fmt.Errorf("missing API reference")
			}
			return expand(doc, target, depth+1)
		}
		out := make(map[string]any, len(v))
		for k, child := range v {
			expanded, err := expand(doc, child, depth+1)
			if err != nil {
				return nil, err
			}
			out[k] = expanded
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			expanded, err := expand(doc, child, depth+1)
			if err != nil {
				return nil, err
			}
			out[i] = expanded
		}
		return out, nil
	default:
		return value, nil
	}
}

func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func array(v any) []any           { a, _ := v.([]any); return a }
func stringValue(v any) string    { s, _ := v.(string); return s }

func (o Operation) Validate(args map[string]any) error {
	if err := o.validator.Validate(args); err != nil {
		return fmt.Errorf("invalid arguments for %s; use its schema to check required fields, types and limits", o.Name)
	}
	return nil
}

func DecodeArguments(data []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var values map[string]any
	if err := decoder.Decode(&values); err != nil || values == nil {
		return nil, errors.New("arguments must be a JSON object")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("arguments must contain one JSON object")
	}
	if _, err := normalizeNumbers(values); err != nil {
		return nil, err
	}
	return values, nil
}

func normalizeNumbers(value any) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			normalized, err := normalizeNumbers(child)
			if err != nil {
				return nil, err
			}
			v[key] = normalized
		}
	case []any:
		for i, child := range v {
			normalized, err := normalizeNumbers(child)
			if err != nil {
				return nil, err
			}
			v[i] = normalized
		}
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return n, nil
		}
		if !strings.ContainsAny(v.String(), ".eE") {
			return nil, errors.New("integer argument exceeds int64")
		}
		n, err := v.Float64()
		if err != nil {
			return nil, errors.New("numeric argument is out of range")
		}
		return n, nil
	}
	return value, nil
}

func operationAliases(definition map[string]any) []string {
	var aliases []string
	for _, raw := range array(definition["x-cli-aliases"]) {
		aliases = append(aliases, stringValue(raw))
	}
	return aliases
}
