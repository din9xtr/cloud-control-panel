package files

import (
	"errors"
	"strings"
	"testing"
)

func TestParsePath(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "root empty", raw: "", want: ""},
		{name: "root slash", raw: "/", want: ""},
		{name: "nested", raw: "/photos/2026/", want: "photos/2026"},
		{name: "unicode", raw: "документы/отчёт.pdf", want: "документы/отчёт.pdf"},
		{name: "spaces inside", raw: "my folder/a b.txt", want: "my folder/a b.txt"},
		{name: "dot dot", raw: "a/../b", wantErr: true},
		{name: "leading dot dot", raw: "../etc/passwd", wantErr: true},
		{name: "dot segment", raw: "a/./b", wantErr: true},
		{name: "double slash", raw: "a//b", wantErr: true},
		{name: "backslash", raw: `a\..\b`, wantErr: true},
		{name: "nul byte", raw: "a\x00b", wantErr: true},
		{name: "newline", raw: "a\nb", wantErr: true},
		{name: "trailing space", raw: "a /b", wantErr: true},
		{name: "invalid utf8", raw: "a\xffb", wantErr: true},
		{name: "too long segment", raw: strings.Repeat("a", 256), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePath(tt.raw)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidPath) {
					t.Fatalf("ParsePath(%q) error = %v, want ErrInvalidPath", tt.raw, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePath(%q) unexpected error: %v", tt.raw, err)
			}
			if got.String() != tt.want {
				t.Fatalf("ParsePath(%q) = %q, want %q", tt.raw, got.String(), tt.want)
			}
		})
	}
}

func TestPathNavigation(t *testing.T) {
	p, err := ParsePath("a/b/c.txt")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "c.txt" {
		t.Errorf("Name() = %q", p.Name())
	}
	if p.Parent().String() != "a/b" {
		t.Errorf("Parent() = %q", p.Parent().String())
	}
	if !Root.Contains(p) {
		t.Error("root must contain every path")
	}
	ab, _ := ParsePath("a/b")
	abc, _ := ParsePath("a/bc")
	if !ab.Contains(p) || ab.Contains(abc) {
		t.Error("Contains must respect segment boundaries")
	}
	joined, err := ab.Join("d")
	if err != nil || joined.String() != "a/b/d" {
		t.Errorf("Join() = %q, %v", joined.String(), err)
	}
	if _, err := ab.Join(".."); !errors.Is(err, ErrInvalidName) {
		t.Errorf("Join(..) error = %v", err)
	}
}

func TestPathRebase(t *testing.T) {
	from, _ := ParsePath("a/b")
	to, _ := ParsePath("x")
	tests := []struct {
		in   string
		to   Path
		want string
	}{
		{in: "a/b", to: to, want: "x"},
		{in: "a/b/c/d", to: to, want: "x/c/d"},
		{in: "a/bc", to: to, want: "a/bc"},
		{in: "a/b/c", to: Root, want: "c"},
	}
	for _, tt := range tests {
		p, _ := ParsePath(tt.in)
		if got := p.Rebase(from, tt.to).String(); got != tt.want {
			t.Errorf("Rebase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestValidateNewNameRejectsBidiControls(t *testing.T) {
	if err := ValidateNewName("invoice\u202Efdp.exe"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("bidi override accepted: %v", err)
	}
	if err := ValidateName("invoice\u202Efdp.exe"); err != nil {
		t.Fatalf("existing names with bidi controls must stay readable: %v", err)
	}
	if err := ValidateNewName("отчёт 2026.pdf"); err != nil {
		t.Fatalf("regular unicode rejected: %v", err)
	}
}
