package api

import _ "embed"

//go:embed openapi.yaml
var _contract string

func Contract() string { return _contract }
