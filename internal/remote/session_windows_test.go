package remote

import (
	"bytes"
	"testing"
)

func TestWindowsSessionProtection(t *testing.T) {
	plain := []byte(`{"token":"invented-session-fixture"}`)
	protected, err := protectSession(plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(protected, []byte("invented-session-fixture")) {
		t.Fatal("session token stored in plaintext")
	}
	restored, err := unprotectSession(protected)
	if err != nil || !bytes.Equal(restored, plain) {
		t.Fatalf("session round trip failed: %v", err)
	}
	protected[len(protected)/2] ^= 0xff
	if _, err := unprotectSession(protected); err == nil {
		t.Fatal("tampered session accepted")
	}
	for _, data := range [][]byte{nil, plain} {
		if _, err := unprotectSession(data); err == nil {
			t.Fatal("unprotected session accepted")
		}
	}
}
