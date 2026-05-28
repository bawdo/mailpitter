package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bawdo/mailpitter/internal/state"
)

// Deregister removes the named consumer from state. Returns a non-empty
// notice (intended for stderr) when the name was not registered. Never
// errors on unknown names.
func (a *App) Deregister(_ context.Context, name string) (string, error) {
	lock, err := state.AcquireLock(a.paths.LockPath, 5*time.Second)
	if err != nil {
		if errors.Is(err, state.ErrLockTimeout) {
			return "", StateLockf("%v", err)
		}
		return "", err
	}
	defer func() { _ = lock.Release() }()

	s, err := state.Load(a.paths.StatePath)
	if err != nil {
		if state.IsCorrupt(err) {
			return "", StateCorruptf("%v", err)
		}
		return "", err
	}
	if _, present := s.Consumers[name]; !present {
		return fmt.Sprintf("no such consumer %q\n", name), nil
	}
	delete(s.Consumers, name)
	if err := state.Save(a.paths.StatePath, s); err != nil {
		return "", err
	}
	return "", nil
}
