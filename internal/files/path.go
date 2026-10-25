package files

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxNameBytes = 255
	maxPathBytes = 4096
)

type Path struct {
	value string
}

var Root = Path{}

func ParsePath(raw string) (Path, error) {
	trimmed := strings.Trim(raw, "/")
	if trimmed == "" {
		return Root, nil
	}
	if len(trimmed) > maxPathBytes {
		return Path{}, ErrInvalidPath
	}
	for segment := range strings.SplitSeq(trimmed, "/") {
		if err := ValidateName(segment); err != nil {
			return Path{}, ErrInvalidPath
		}
	}
	return Path{value: trimmed}, nil
}

func ValidateName(name string) error {
	if name == "" || name == "." || name == ".." || len(name) > maxNameBytes {
		return ErrInvalidName
	}
	if !utf8.ValidString(name) || strings.TrimSpace(name) != name {
		return ErrInvalidName
	}
	for _, r := range name {
		if r == '/' || r == '\\' || unicode.IsControl(r) {
			return ErrInvalidName
		}
	}
	return nil
}

func ValidateNewName(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	for _, r := range name {
		if isBidiControl(r) {
			return ErrInvalidName
		}
	}
	return nil
}

func isBidiControl(r rune) bool {
	switch {
	case r == '\u061C', r == '\u200E', r == '\u200F':
		return true
	case r >= '\u202A' && r <= '\u202E':
		return true
	case r >= '\u2066' && r <= '\u2069':
		return true
	}
	return false
}

func (p Path) String() string {
	return p.value
}

func (p Path) IsRoot() bool {
	return p.value == ""
}

func (p Path) Name() string {
	if i := strings.LastIndexByte(p.value, '/'); i >= 0 {
		return p.value[i+1:]
	}
	return p.value
}

func (p Path) Parent() Path {
	if i := strings.LastIndexByte(p.value, '/'); i >= 0 {
		return Path{value: p.value[:i]}
	}
	return Root
}

func (p Path) Join(name string) (Path, error) {
	if err := ValidateName(name); err != nil {
		return Path{}, err
	}
	if p.IsRoot() {
		return Path{value: name}, nil
	}
	return Path{value: p.value + "/" + name}, nil
}

func (p Path) Contains(other Path) bool {
	if p.IsRoot() {
		return true
	}
	return other.value == p.value || strings.HasPrefix(other.value, p.value+"/")
}

func (p Path) Rebase(from, to Path) Path {
	if !from.Contains(p) {
		return p
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(p.value, from.value), "/")
	switch {
	case rest == "":
		return to
	case to.IsRoot():
		return Path{value: rest}
	default:
		return Path{value: to.value + "/" + rest}
	}
}

func (p Path) fsPath() string {
	if p.IsRoot() {
		return "."
	}
	return p.value
}
