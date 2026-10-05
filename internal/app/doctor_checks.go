package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/bawdo/mailpitter/internal/caddyshack"
	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/state"
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

// dialTCP is a seam over the reachability probe so tests stay off the
// network.
var dialTCP = func(addr string) error {
	c, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return err
	}
	return c.Close()
}

// checkSMTPGatewayBinding confirms that, on a native Linux engine, the
// container publishes SMTP on the docker0 gateway as well as loopback, so
// other containers can reach it via host.docker.internal.
func checkSMTPGatewayBinding(a *App) CheckResult {
	const name = "smtp-gateway-binding"
	hn := a.hostNetwork()
	switch hn.Kind {
	case EngineLoopbackForwarded:
		return CheckResult{Name: name, Status: SeveritySkip, Summary: "not needed: " + hn.Reason + " forwards host.docker.internal to loopback"}
	case EngineUnknown:
		return CheckResult{Name: name, Status: SeveritySkip, Summary: "engine not classified; loopback-only bindings kept", Detail: []string{hn.Reason}}
	}
	st, err := a.docker.ContainerState(ContainerName)
	if err != nil || st == docker.StateAbsent {
		return CheckResult{Name: name, Status: SeveritySkip, Summary: "container not installed"}
	}
	bindings, err := a.docker.ContainerPortBindings(ContainerName)
	if err != nil {
		return CheckResult{Name: name, Status: SeverityFail, Summary: err.Error()}
	}
	addr := net.JoinHostPort(hn.Gateway, strconv.Itoa(SMTPPort))
	if smtpGatewayMissing(hn, bindings) {
		return CheckResult{
			Name: name, Status: SeverityFail,
			Summary: addr + " not published; containers cannot reach SMTP via host.docker.internal",
			Detail:  []string{"the container predates the gateway binding; install recreates it (persistent mode keeps stored mail)"},
			Fix:     &FixHint{Label: "mailpitter install", Command: "mailpitter install"},
		}
	}
	return CheckResult{Name: name, Status: SeverityPass, Summary: addr + " published"}
}

// checkSMTPGatewayReachable dials the gateway-published SMTP port from the
// host and reports whether ufw will admit container traffic to it. The
// host-side dial proves the port is listening but not the firewall path
// (host-originated traffic to its own docker0 address bypasses ufw's
// input rules), hence the separate rule check.
func checkSMTPGatewayReachable(a *App) CheckResult {
	const name = "smtp-gateway-reachable"
	hn := a.hostNetwork()
	if hn.Kind != EngineNativeLinux {
		return CheckResult{Name: name, Status: SeveritySkip, Summary: "not a native Linux engine"}
	}
	addr := net.JoinHostPort(hn.Gateway, strconv.Itoa(SMTPPort))
	if err := dialTCP(addr); err != nil {
		return CheckResult{Name: name, Status: SeverityFail, Summary: addr + " not listening: " + err.Error()}
	}
	ufw, err := readUFWStatus(hn.Gateway)
	switch {
	case err != nil:
		return CheckResult{
			Name: name, Status: SeverityWarn,
			Summary: addr + " listening; could not read ufw config",
			Detail:  []string{err.Error(), "if ufw is active, run: " + ufwRule(hn.Gateway)},
		}
	case !ufw.Active:
		return CheckResult{Name: name, Status: SeverityPass, Summary: addr + " listening; ufw not active"}
	case ufw.RuleSource == "":
		return CheckResult{
			Name: name, Status: SeverityWarn,
			Summary: addr + " listening; ufw is active and has no rule admitting containers",
			Detail:  []string{"containers will time out connecting to host.docker.internal:1025", "run: " + ufwRule(hn.Gateway)},
		}
	}
	return CheckResult{Name: name, Status: SeverityPass, Summary: addr + " listening; ufw allows from " + ufw.RuleSource}
}

// checkCaddyShackInstalled reports whether the caddy_shack binary is on
// $PATH. Uses exec.LookPath via the caddyshack client - the previous
// Status-probing approach falsely PASSed when caddy_shack was genuinely
// missing, because ENOENT from exec.CommandContext doesn't surface as
// caddyshack.ErrNotInstalled.
func checkCaddyShackInstalled(a *App) CheckResult {
	if err := a.caddy.Installed(); err != nil {
		return CheckResult{
			Name: "caddy_shack-installed", Status: SeverityFail,
			Summary: "caddy_shack not installed",
			Detail:  []string{"install via: go install github.com/bawdo/caddy_shack@latest"},
		}
	}
	return CheckResult{Name: "caddy_shack-installed", Status: SeverityPass, Summary: "installed"}
}

// checkCaddyShackRunning confirms the caddy_shack container is up and the
// admin API is reachable. Skips when caddy_shack itself is not installed.
// When caddy_shack returns ErrUnhealthy it still emits structured state,
// so we use the parsed Status fields to report the specific reason.
func checkCaddyShackRunning(a *App) CheckResult {
	st, err := a.caddy.Status(context.Background())
	if errors.Is(err, caddyshack.ErrNotInstalled) {
		return CheckResult{Name: "caddy_shack-running", Status: SeveritySkip, Summary: "caddy_shack not installed"}
	}
	if errors.Is(err, caddyshack.ErrUnhealthy) {
		return CheckResult{
			Name: "caddy_shack-running", Status: SeverityFail,
			Summary: fmt.Sprintf("caddy_shack unhealthy (container=%s, admin_reachable=%v)", st.ContainerState, st.AdminReachable),
			Fix:     &FixHint{Label: "caddy_shack start", Command: "caddy_shack start"},
		}
	}
	if err != nil {
		return CheckResult{
			Name: "caddy_shack-running", Status: SeverityFail,
			Summary: fmt.Sprintf("caddy_shack status failed: %v", err),
		}
	}
	if st.Healthy {
		return CheckResult{
			Name: "caddy_shack-running", Status: SeverityPass,
			Summary: fmt.Sprintf("running (%d backends)", st.BackendCount),
		}
	}
	return CheckResult{
		Name: "caddy_shack-running", Status: SeverityFail,
		Summary: fmt.Sprintf("caddy_shack reports unhealthy (container=%s, admin_reachable=%v)", st.ContainerState, st.AdminReachable),
		Fix:     &FixHint{Label: "caddy_shack start", Command: "caddy_shack start"},
	}
}

// checkMailpitImage verifies the mailpit Docker image has been pulled locally.
func checkMailpitImage(a *App) CheckResult {
	present, err := a.docker.ImagePresent(Image)
	if err != nil {
		return CheckResult{Name: "mailpit-image", Status: SeverityFail, Summary: err.Error()}
	}
	if present {
		return CheckResult{Name: "mailpit-image", Status: SeverityPass, Summary: "image present"}
	}
	return CheckResult{
		Name: "mailpit-image", Status: SeverityFail,
		Summary: "image not pulled",
		Fix:     &FixHint{Label: "docker pull " + Image, Command: "docker pull " + Image},
	}
}

// checkCaddyRoute confirms mailpit.test is registered as a backend in
// caddy_shack. Uses `caddy_shack list --json`, which works even when
// caddy itself is stopped (the route may still be persisted in
// caddy_shack's state).
func checkCaddyRoute(a *App) CheckResult {
	backends, err := a.caddy.List(context.Background())
	if errors.Is(err, caddyshack.ErrNotInstalled) {
		return CheckResult{Name: "caddy-route", Status: SeveritySkip, Summary: "caddy_shack not installed"}
	}
	if err != nil {
		return CheckResult{Name: "caddy-route", Status: SeveritySkip, Summary: fmt.Sprintf("caddy_shack list failed: %v", err)}
	}
	for _, b := range backends {
		if b.Name == "mailpit" && b.Domain == Domain {
			return CheckResult{Name: "caddy-route", Status: SeverityPass, Summary: "mailpit.test route registered"}
		}
	}
	return CheckResult{
		Name: "caddy-route", Status: SeverityFail,
		Summary: "mailpit.test not registered",
		Fix:     &FixHint{Label: "mailpitter start (re-registers)", Command: "mailpitter start"},
	}
}

// checkInstallModeMatch is an informational check that reads the recorded
// install mode from state. It only reports PASS or SKIP - a deeper check
// (e.g. docker inspect to verify volume mounts) is a potential follow-up.
func checkInstallModeMatch(a *App) CheckResult {
	s, err := state.Load(a.paths.StatePath)
	if err != nil || s.Install == nil {
		return CheckResult{Name: "install-mode", Status: SeveritySkip, Summary: "no install recorded"}
	}
	return CheckResult{
		Name: "install-mode", Status: SeverityPass,
		Summary: fmt.Sprintf("recorded mode=%s", s.Install.Mode),
	}
}
