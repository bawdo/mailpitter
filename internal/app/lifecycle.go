package app

import (
	"context"
	"fmt"

	"github.com/bawdo/mailpitter/internal/docker"
)

// Start brings the container up (no-op if already running) and re-registers
// mailpit.test with caddy_shack so the route is current.
func (a *App) Start(ctx context.Context) error {
	if err := a.docker.DaemonReachable(); err != nil {
		return DockerUnreachablef("%v", err)
	}
	st, err := a.docker.ContainerState(ContainerName)
	if err != nil {
		return err
	}
	switch st {
	case docker.StateAbsent:
		return MailpitNotInstalledf("container %q does not exist; run: mailpitter install", ContainerName)
	case docker.StateStopped:
		if err := a.docker.Start(ContainerName); err != nil {
			return err
		}
	case docker.StateRunning:
		// no-op
	}
	if err := a.caddy.Register(ctx, "mailpit", Domain, UIPort); err != nil {
		return CaddyShackf(err, "register %s", Domain)
	}
	_, _ = fmt.Fprintln(a.notices, "registered mailpit.test with caddy_shack")
	return nil
}

// Stop deregisters mailpit.test with caddy_shack first (so no dangling
// 502 route remains), then stops the container. Both steps are idempotent.
func (a *App) Stop(ctx context.Context) error {
	if err := a.caddy.Deregister(ctx, "mailpit"); err != nil {
		return CaddyShackf(err, "deregister mailpit")
	}
	_, _ = fmt.Fprintln(a.notices, "deregistered mailpit.test from caddy_shack")
	if err := a.docker.DaemonReachable(); err != nil {
		return DockerUnreachablef("%v", err)
	}
	st, err := a.docker.ContainerState(ContainerName)
	if err != nil {
		return err
	}
	if st == docker.StateRunning {
		if err := a.docker.Stop(ContainerName); err != nil {
			return err
		}
	}
	return nil
}

// Restart is Stop then Start.
func (a *App) Restart(ctx context.Context) error {
	if err := a.Stop(ctx); err != nil {
		return err
	}
	return a.Start(ctx)
}
