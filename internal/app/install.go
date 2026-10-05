package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

	hn := a.hostNetwork()
	a.noteUnknownEngine(hn)

	switch st {
	case docker.StateRunning, docker.StateStopped:
		recreated, err := a.ensureSMTPGateway(s, wantMode, hn)
		if err != nil {
			return err
		}
		if !recreated && st == docker.StateStopped {
			if err := a.docker.Start(ContainerName); err != nil {
				return err
			}
		}
	case docker.StateAbsent:
		if err := a.docker.PullImage(Image); err != nil {
			return err
		}
		if !in.Ephemeral {
			if err := a.docker.EnsureVolume(DataVolume); err != nil {
				return err
			}
		}
		if err := a.docker.CreateAndStart(containerSpec(wantMode, Image, hn.Gateway)); err != nil {
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
	_, _ = fmt.Fprintln(a.notices, "registered mailpit.test with caddy_shack")
	a.noteUFW(hn)
	return nil
}

// ensureSMTPGateway recreates an existing container that predates the
// docker0-gateway SMTP binding (port bindings cannot be changed in place).
// It only recreates when stored mail survives: persistent mode with the
// data volume actually mounted. Otherwise it explains how to recreate by
// hand and leaves the container alone. Reports whether it recreated.
func (a *App) ensureSMTPGateway(s *state.State, wantMode state.Mode, hn HostNetwork) (bool, error) {
	bindings, err := a.docker.ContainerPortBindings(ContainerName)
	if err != nil {
		return false, err
	}
	if !smtpGatewayMissing(hn, bindings) {
		return false, nil
	}
	mounts, err := a.docker.ContainerMounts(ContainerName)
	if err != nil {
		return false, err
	}
	mode, image := wantMode, Image
	if s.Install != nil {
		mode = s.Install.Mode
		if s.Install.Image != "" {
			image = s.Install.Image
		}
	}
	if mode != state.ModePersistent || !hasDataVolume(mounts) {
		_, _ = fmt.Fprintf(a.notices,
			"container %q does not publish SMTP on %s:%d, so other containers cannot reach it via host.docker.internal.\n"+
				"not recreating it automatically: its messages are not on the %s volume and would be lost. To recreate anyway:\n"+
				"  %s\n",
			ContainerName, hn.Gateway, SMTPPort, DataVolume,
			strings.TrimSpace("docker rm -f "+ContainerName+" && mailpitter install "+flagFor(mode)))
		return false, nil
	}

	lock, err := state.AcquireLock(a.paths.LockPath, 5*time.Second)
	if err != nil {
		if errors.Is(err, state.ErrLockTimeout) {
			return false, StateLockf("%v", err)
		}
		return false, err
	}
	defer func() { _ = lock.Release() }()

	_, _ = fmt.Fprintf(a.notices, "recreating %q to publish SMTP on %s:%d (messages kept in volume %s)\n",
		ContainerName, hn.Gateway, SMTPPort, DataVolume)
	if err := a.recreateContainer(containerSpec(mode, image, hn.Gateway)); err != nil {
		return false, err
	}
	return true, nil
}

// recreateContainer replaces the mailpit container with one built from
// spec, ensuring the data volume when spec mounts it and reattaching every
// network the old container was on. `docker run` only attaches the default
// bridge, and other tooling relies on mailpitter staying on networks such
// as caddy_shack, which mailpitter itself never attaches.
func (a *App) recreateContainer(spec docker.ContainerSpec) error {
	nets, err := a.docker.ContainerNetworks(ContainerName)
	if err != nil {
		return err
	}
	if err := a.docker.Remove(ContainerName); err != nil {
		return err
	}
	for _, v := range spec.Volumes {
		if err := a.docker.EnsureVolume(v.VolumeName); err != nil {
			return err
		}
	}
	if err := a.docker.CreateAndStart(spec); err != nil {
		return err
	}
	for _, n := range nets {
		if n == "bridge" {
			continue
		}
		if err := a.docker.ConnectNetwork(n, ContainerName); err != nil {
			return err
		}
	}
	return nil
}

func hasDataVolume(mounts []docker.VolumeMount) bool {
	for _, m := range mounts {
		if m.VolumeName == DataVolume && m.ContainerPath == "/data" {
			return true
		}
	}
	return false
}

// containerSpec builds the docker ContainerSpec for the mailpit container
// in the given mode against the given image. Volume creation is the
// caller's responsibility (gated on persistent mode).
//
// smtpGateway is the docker0 bridge gateway on a native Linux engine, or
// "" elsewhere. When set, SMTP is published there too so containers can
// reach it via host.docker.internal (host-gateway). The UI stays on
// loopback only, and nothing is ever bound to 0.0.0.0.
func containerSpec(mode state.Mode, image, smtpGateway string) docker.ContainerSpec {
	ports := []docker.PortMapping{{HostAddr: HostBindAddr, HostPort: SMTPPort, ContainerPort: SMTPPort}}
	if smtpGateway != "" {
		ports = append(ports, docker.PortMapping{HostAddr: smtpGateway, HostPort: SMTPPort, ContainerPort: SMTPPort})
	}
	ports = append(ports, docker.PortMapping{HostAddr: HostBindAddr, HostPort: UIPort, ContainerPort: UIPort})
	spec := docker.ContainerSpec{
		Name:         ContainerName,
		Image:        image,
		Env:          []string{"MP_DATABASE=/data/mailpit.db"},
		PortMappings: ports,
	}
	if mode == state.ModePersistent {
		spec.Volumes = []docker.VolumeMount{
			{VolumeName: DataVolume, ContainerPath: "/data"},
		}
	} else {
		// Ephemeral: drop MP_DATABASE so mailpit uses its in-memory store
		// rather than writing to /data.
		spec.Env = nil
	}
	return spec
}

func flagFor(m state.Mode) string {
	if m == state.ModeEphemeral {
		return "--ephemeral"
	}
	return ""
}
