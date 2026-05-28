package app

import (
	"context"
	"errors"
	"time"

	"github.com/bawdo/mailpitter/internal/caddyshack"
	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/state"
)

// InstallInput holds options for the install command.
type InstallInput struct {
	Ephemeral bool
}

// Install pulls the pinned image, creates the data volume (unless
// Ephemeral), creates and starts the container, records install state,
// and registers mailpit.test with caddy_shack. Idempotent: re-running
// against a healthy install only re-issues the caddy_shack registration.
func (a *App) Install(ctx context.Context, in InstallInput) error {
	if err := a.docker.DaemonReachable(); err != nil {
		return DockerUnreachablef("%v", err)
	}

	// Reject mode switches against an existing install.
	s, err := state.Load(a.paths.StatePath)
	if err != nil {
		if state.IsCorrupt(err) {
			return StateCorruptf("%v", err)
		}
		return err
	}
	wantMode := state.ModePersistent
	if in.Ephemeral {
		wantMode = state.ModeEphemeral
	}
	if s.Install != nil && s.Install.Mode != wantMode {
		return InvalidArgsf(
			"cannot switch install mode from %s to %s; run: mailpitter reset CONFIRM=yes && mailpitter install %s",
			s.Install.Mode, wantMode, flagFor(wantMode))
	}

	st, err := a.docker.ContainerState(ContainerName)
	if err != nil {
		return err
	}

	switch st {
	case docker.StateRunning:
		// Already up; just re-register with caddy_shack so the route is healthy.
	case docker.StateStopped:
		if err := a.docker.Start(ContainerName); err != nil {
			return err
		}
	case docker.StateAbsent:
		if err := a.docker.PullImage(Image); err != nil {
			return err
		}
		spec := docker.ContainerSpec{
			Name:  ContainerName,
			Image: Image,
			Env:   []string{"MP_DATABASE=/data/mailpit.db"},
			PortMappings: []docker.PortMapping{
				{HostAddr: HostBindAddr, HostPort: SMTPPort, ContainerPort: SMTPPort},
				{HostAddr: HostBindAddr, HostPort: UIPort, ContainerPort: UIPort},
			},
		}
		if !in.Ephemeral {
			if err := a.docker.EnsureVolume(DataVolume); err != nil {
				return err
			}
			spec.Volumes = []docker.VolumeMount{
				{VolumeName: DataVolume, ContainerPath: "/data"},
			}
		} else {
			// Ephemeral: drop the MP_DATABASE env so mailpit uses its
			// in-memory store rather than trying to write to /data.
			spec.Env = nil
		}
		if err := a.docker.CreateAndStart(spec); err != nil {
			_ = a.docker.Remove(ContainerName)
			return err
		}
	}

	// Record install state.
	if s.Install == nil {
		s.Install = &state.Install{
			Mode:        wantMode,
			Image:       Image,
			InstalledAt: time.Now().Format(time.RFC3339),
		}
		if err := state.Save(a.paths.StatePath, s); err != nil {
			return err
		}
	}

	// Register UI route with caddy_shack.
	if err := a.caddy.Register(ctx, "mailpit", Domain, UIPort); err != nil {
		if errors.Is(err, caddyshack.ErrNotInstalled) {
			return Prerequisitef("caddy_shack not installed; run: caddy_shack install")
		}
		return CaddyShackf(err, "register %s", Domain)
	}
	return nil
}

func flagFor(m state.Mode) string {
	if m == state.ModeEphemeral {
		return "--ephemeral"
	}
	return ""
}
