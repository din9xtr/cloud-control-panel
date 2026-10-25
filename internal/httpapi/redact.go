package httpapi

import (
	"cmp"
	"io/fs"
	"os"
	"slices"
	"strings"
)

const redactedPath = "[path]"

func redact(err error) string {
	msg := err.Error()
	paths := errorPaths(err)
	slices.SortFunc(paths, func(a, b string) int { return cmp.Compare(len(b), len(a)) })
	for _, p := range paths {
		msg = strings.ReplaceAll(msg, p, redactedPath)
	}
	return msg
}

func errorPaths(err error) []string {
	var paths []string
	pending := []error{err}
	for len(pending) > 0 {
		next := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		switch e := any(next).(type) {
		case *fs.PathError:
			paths = appendPath(paths, e.Path)
		case *os.LinkError:
			paths = appendPath(paths, e.Old, e.New)
		}
		switch u := any(next).(type) {
		case interface{ Unwrap() error }:
			if inner := u.Unwrap(); inner != nil {
				pending = append(pending, inner)
			}
		case interface{ Unwrap() []error }:
			pending = append(pending, u.Unwrap()...)
		}
	}
	return paths
}

func appendPath(paths []string, candidates ...string) []string {
	for _, p := range candidates {
		if p != "" && p != "." {
			paths = append(paths, p)
		}
	}
	return paths
}
