package app

import (
	"context"

	"github.com/bawdo/mailpitter/internal/state"
)

// StatusReport is the snapshot returned by Status.
type StatusReport struct {
	Container      string `json:"container"`
	APIReachable   bool   `json:"api_reachable"`
	MailpitVersion string `json:"mailpit_version,omitempty"`
	Messages       int    `json:"messages"`
	InstallMode    string `json:"install_mode,omitempty"`
	Domain         string `json:"domain"`
}

// Status returns a snapshot of container + admin-API health and install state.
func (a *App) Status(_ context.Context) (StatusReport, error) {
	rep := StatusReport{Domain: Domain}

	st, err := a.docker.ContainerState(ContainerName)
	if err != nil {
		return rep, err
	}
	rep.Container = st.String()

	info, err := a.mailpit.Info()
	if err == nil {
		rep.APIReachable = true
		rep.MailpitVersion = info.Version
		rep.Messages = info.Messages
	}

	s, err := state.Load(a.paths.StatePath)
	if err == nil && s.Install != nil {
		rep.InstallMode = string(s.Install.Mode)
	}
	return rep, nil
}
