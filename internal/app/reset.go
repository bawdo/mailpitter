package app

import (
	"context"
	"errors"
	"time"

	"github.com/bawdo/mailpitter/internal/state"
)

// Reset stops the container, removes it, removes the data volume, and
// clears the install block from state. Consumers are preserved.
func (a *App) Reset(ctx context.Context) error {
	if err := a.Stop(ctx); err != nil {
		return err
	}
	if err := a.docker.Remove(ContainerName); err != nil {
		return err
	}
	if err := a.docker.RemoveVolume(DataVolume); err != nil {
		return err
	}

	lock, err := state.AcquireLock(a.paths.LockPath, 5*time.Second)
	if err != nil {
		if errors.Is(err, state.ErrLockTimeout) {
			return StateLockf("%v", err)
		}
		return err
	}
	defer func() { _ = lock.Release() }()

	s, err := state.Load(a.paths.StatePath)
	if err != nil {
		if state.IsCorrupt(err) {
			return StateCorruptf("%v", err)
		}
		return err
	}
	s.Install = nil
	return state.Save(a.paths.StatePath, s)
}
