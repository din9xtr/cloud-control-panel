package auth

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Hasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type BcryptHasher struct {
	Cost int
}

var _ Hasher = BcryptHasher{}

func (h BcryptHasher) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.Cost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (h BcryptHasher) Compare(hash, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return ErrInvalidCredentials
	}
	return err
}

type BoundedHasher struct {
	next  Hasher
	slots chan struct{}
	wait  time.Duration
}

var _ Hasher = (*BoundedHasher)(nil)

func NewBoundedHasher(next Hasher, concurrency int, wait time.Duration) *BoundedHasher {
	return &BoundedHasher{next: next, slots: make(chan struct{}, concurrency), wait: wait}
}

func (h *BoundedHasher) Hash(password string) (string, error) {
	release, err := h.acquire()
	if err != nil {
		return "", err
	}
	defer release()
	return h.next.Hash(password)
}

func (h *BoundedHasher) Compare(hash, password string) error {
	release, err := h.acquire()
	if err != nil {
		return err
	}
	defer release()
	return h.next.Compare(hash, password)
}

func (h *BoundedHasher) acquire() (func(), error) {
	timer := time.NewTimer(h.wait)
	defer timer.Stop()
	select {
	case h.slots <- struct{}{}:
		return func() { <-h.slots }, nil
	case <-timer.C:
		return nil, ErrBusy
	}
}
