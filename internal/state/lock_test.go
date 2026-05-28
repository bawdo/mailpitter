package state

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestAcquireAndRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	l, err := AcquireLock(path, 1*time.Second)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	if err := l.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestAcquireContendedTimesOut(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	first, err := AcquireLock(path, 1*time.Second)
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}
	defer func() { _ = first.Release() }()

	_, err = AcquireLock(path, 100*time.Millisecond)
	if !errors.Is(err, ErrLockTimeout) {
		t.Fatalf("second AcquireLock should return ErrLockTimeout, got %v", err)
	}
}

func TestAcquireAfterReleaseSucceeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		l, err := AcquireLock(path, 1*time.Second)
		if err != nil {
			t.Errorf("inner Acquire: %v", err)
			return
		}
		time.Sleep(50 * time.Millisecond)
		_ = l.Release()
	}()
	time.Sleep(10 * time.Millisecond) // let goroutine grab first
	l, err := AcquireLock(path, 1*time.Second)
	if err != nil {
		t.Fatalf("blocked Acquire: %v", err)
	}
	_ = l.Release()
	wg.Wait()
}
