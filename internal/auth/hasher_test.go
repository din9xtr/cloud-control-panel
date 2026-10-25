package auth

import (
	"errors"
	"testing"
	"time"
)

type blockingHasher struct {
	release chan struct{}
}

func (h blockingHasher) Hash(string) (string, error) {
	<-h.release
	return "hash", nil
}

func (h blockingHasher) Compare(string, string) error {
	<-h.release
	return nil
}

func TestBoundedHasherShedsLoad(t *testing.T) {
	inner := blockingHasher{release: make(chan struct{})}
	h := NewBoundedHasher(inner, 1, 20*time.Millisecond)
	done := make(chan error, 1)
	go func() { done <- h.Compare("hash", "pw") }()
	time.Sleep(5 * time.Millisecond)
	if err := h.Compare("hash", "pw"); !errors.Is(err, ErrBusy) {
		t.Fatalf("second Compare() error = %v, want ErrBusy", err)
	}
	close(inner.release)
	if err := <-done; err != nil {
		t.Fatalf("first Compare() error = %v", err)
	}
	if _, err := h.Hash("pw"); err != nil {
		t.Fatalf("Hash() after release error = %v", err)
	}
}
