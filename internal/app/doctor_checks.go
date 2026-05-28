package app

import (
	"fmt"
	"net"
	"strconv"

	"github.com/bawdo/mailpitter/internal/docker"
)

// Severity is the result of a single doctor check.
type Severity int

const (
	SeverityPass Severity = iota
	SeverityWarn
	SeverityFail
	SeveritySkip
)

func (s Severity) String() string {
	switch s {
	case SeverityPass:
		return "PASS"
	case SeverityWarn:
		return "WARN"
	case SeverityFail:
		return "FAIL"
	case SeveritySkip:
		return "SKIP"
	default:
		return "????"
	}
}

// CheckResult is one row of the doctor report.
type CheckResult struct {
	Name    string   `json:"name"`
	Status  Severity `json:"status"`
	Summary string   `json:"summary"`
	Detail  []string `json:"detail,omitempty"`
	Fix     *FixHint `json:"fix,omitempty"`
}

// FixHint describes an automated fix the `--fix` runner can apply.
type FixHint struct {
	Label   string `json:"label"`
	Command string `json:"command"` // bash one-liner shown on screen
}

func checkDockerDaemon(a *App) CheckResult {
	if err := a.docker.DaemonReachable(); err != nil {
		return CheckResult{
			Name: "docker-daemon", Status: SeverityFail,
			Summary: "docker daemon unreachable",
			Detail:  []string{err.Error(), "start Docker Desktop and rerun"},
		}
	}
	return CheckResult{Name: "docker-daemon", Status: SeverityPass, Summary: "reachable"}
}

func checkMailpitContainer(a *App) CheckResult {
	st, err := a.docker.ContainerState(ContainerName)
	if err != nil {
		return CheckResult{Name: "mailpit-container", Status: SeverityFail, Summary: err.Error()}
	}
	switch st {
	case docker.StateRunning:
		return CheckResult{Name: "mailpit-container", Status: SeverityPass, Summary: "running"}
	case docker.StateStopped:
		return CheckResult{
			Name: "mailpit-container", Status: SeverityFail,
			Summary: "container exists but is stopped",
			Fix:     &FixHint{Label: "mailpitter start", Command: "mailpitter start"},
		}
	default:
		return CheckResult{
			Name: "mailpit-container", Status: SeverityFail,
			Summary: "container not installed",
			Fix:     &FixHint{Label: "mailpitter install", Command: "mailpitter install"},
		}
	}
}

func checkMailpitAPI(a *App) CheckResult {
	st, err := a.docker.ContainerState(ContainerName)
	if err != nil || st != docker.StateRunning {
		return CheckResult{Name: "mailpit-api", Status: SeveritySkip, Summary: "container not running"}
	}
	info, err := a.mailpit.Info()
	if err != nil {
		return CheckResult{Name: "mailpit-api", Status: SeverityFail, Summary: err.Error()}
	}
	return CheckResult{
		Name: "mailpit-api", Status: SeverityPass,
		Summary: fmt.Sprintf("ok (version=%s, messages=%d)", info.Version, info.Messages),
	}
}

func checkPortFree(name string, port int) CheckResult {
	addr := net.JoinHostPort(HostBindAddr, strconv.Itoa(port))
	l, err := net.Listen("tcp", addr)
	if err == nil {
		_ = l.Close()
		return CheckResult{Name: name, Status: SeverityPass, Summary: addr + " free or held by mailpitter"}
	}
	return CheckResult{
		Name: name, Status: SeverityWarn,
		Summary: addr + " in use",
		Detail:  []string{"may be mailpitter itself (expected after install) or another process; verify with: lsof -nP -iTCP:" + strconv.Itoa(port) + " -sTCP:LISTEN"},
	}
}

func checkPortSMTP(_ *App) CheckResult { return checkPortFree("port-1025", SMTPPort) }
func checkPortUI(_ *App) CheckResult   { return checkPortFree("port-8025", UIPort) }
