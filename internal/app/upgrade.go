package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/state"
)

// Upgrade pulls axllent/mailpit:latest and, if it differs from the image
// the current container runs, recreates the container against it
// (preserving the data volume), records the new image in state, and
// re-registers mailpit.test with caddy_shack. If the pulled image matches
// what is already running, it reports "already up to date" and leaves the
// container untouched.
func (a *App) Upgrade(ctx context.Context) error {
	if err := a.docker.DaemonReachable(); err != nil {
		return DockerUnreachablef("%v", err)
	}

	s, err := state.Load(a.paths.StatePath)
	if err != nil {
		if state.IsCorrupt(err) {
			return StateCorruptf("%v", err)
		}
		return err
	}
	if s.Install == nil {
		return MailpitNotInstalledf("no install recorded; run: mailpitter install")
	}

	st, err := a.docker.ContainerState(ContainerName)
	if err != nil {
		return err
	}
	if st == docker.StateAbsent {
		return MailpitNotInstalledf("container %q does not exist; run: mailpitter install", ContainerName)
	}

	if err := a.docker.PullImage(LatestImage); err != nil {
		return err
	}

	newID, err := a.docker.ImageID(LatestImage)
	if err != nil {
		return err
	}
	curID, err := a.docker.ContainerImageID(ContainerName)
	if err != nil {
		return err
	}
	if newID == curID {
		if v, ok := a.mailpitVersion(); ok {
			_, _ = fmt.Fprintf(a.notices, "already up to date (%s)\n", v)
		} else {
			_, _ = fmt.Fprintln(a.notices, "already up to date")
		}
		return nil
	}

	lock, err := state.AcquireLock(a.paths.LockPath, 5*time.Second)
	if err != nil {
		if errors.Is(err, state.ErrLockTimeout) {
			return StateLockf("%v", err)
		}
		return err
	}
	defer func() { _ = lock.Release() }()

	if err := a.docker.Remove(ContainerName); err != nil {
		return err
	}
	if s.Install.Mode == state.ModePersistent {
		if err := a.docker.EnsureVolume(DataVolume); err != nil {
			return err
		}
	}
	if err := a.docker.CreateAndStart(containerSpec(s.Install.Mode, LatestImage)); err != nil {
		return err
	}

	s.Install.Image = LatestImage
	if err := state.Save(a.paths.StatePath, s); err != nil {
		return err
	}

	if err := a.caddy.Register(ctx, "mailpit", Domain, UIPort); err != nil {
		return CaddyShackf(err, "register %s", Domain)
	}
	_, _ = fmt.Fprintln(a.notices, "registered mailpit.test with caddy_shack")

	if v, ok := a.mailpitVersion(); ok {
		_, _ = fmt.Fprintf(a.notices, "upgraded to %s\n", v)
	} else {
		_, _ = fmt.Fprintln(a.notices, "upgrade complete; run mailpitter status to confirm version")
	}
	return nil
}

// mailpitVersion returns the running mailpit version via the admin API.
// Best-effort: a single call, no retry. The bool is false when the API
// is unreachable (e.g. right after a recreate, before mailpit is ready).
func (a *App) mailpitVersion() (string, bool) {
	info, err := a.mailpit.Info()
	if err != nil {
		return "", false
	}
	return info.Version, true
}
