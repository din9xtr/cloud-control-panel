package remote

import (
	"os"
	"testing"
)

func TestCommandReferenceMatchesContract(t *testing.T) {
	want, err := CommandReference()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("../../docs/commands.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatal("command reference is stale; run go run ./cmd/cloud-docs")
	}
}
