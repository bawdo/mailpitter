//go:build integration

package integration

import (
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func have(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// prerequisite skips the test unless MAILPITTER_INTEGRATION_STRICT is set,
// in which case a missing prerequisite is a failure: CI installs them all,
// so a skip there would hide a broken install step.
func prerequisite(t *testing.T, ok bool, format string, args ...any) {
	t.Helper()
	if ok {
		return
	}
	if os.Getenv("MAILPITTER_INTEGRATION_STRICT") != "" {
		t.Fatalf("strict: "+format, args...)
	}
	t.Skipf(format, args...)
}

func TestMailpitterRoundTrip(t *testing.T) {
	prerequisite(t, have("docker"), "docker not on PATH")
	if have("docker") {
		err := exec.Command("docker", "info").Run()
		prerequisite(t, err == nil, "docker daemon unreachable: %v", err)
	}
	prerequisite(t, have("caddy_shack"), "caddy_shack not on PATH")
	prerequisite(t, have("mailpitter"), "mailpitter not on PATH (run `make install` first)")
	if have("caddy_shack") {
		err := exec.Command("caddy_shack", "status").Run()
		prerequisite(t, err == nil, "caddy_shack not installed or not running; run: caddy_shack install")
	}

	// Install
	if out, err := exec.Command("mailpitter", "install").CombinedOutput(); err != nil {
		t.Fatalf("install: %v: %s", err, out)
	}

	// Status reports running
	out, err := exec.Command("mailpitter", "status", "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("status: %v: %s", err, out)
	}
	if !strings.Contains(string(out), `"container": "running"`) {
		t.Errorf("status JSON missing running container: %s", out)
	}

	// SMTP port reachable
	if !portReachable("127.0.0.1:1025", 5*time.Second) {
		t.Errorf("127.0.0.1:1025 not reachable")
	}

	// Register a consumer; expect snippet on stdout, notice on stderr
	cmd := exec.Command("mailpitter", "register", "smoke", "--service", "smokesvc")
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if !strings.Contains(string(stdout), "EMAIL_HOST=mailpit.test") {
		t.Errorf("register stdout missing env snippet: %s", stdout)
	}

	// List should include it
	out, err = exec.Command("mailpitter", "list", "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("list: %v: %s", err, out)
	}
	if !strings.Contains(string(out), `"name": "smoke"`) {
		t.Errorf("list missing smoke consumer: %s", out)
	}

	// Doctor passes against a healthy install. This is the regression
	// gate for the caddyshack JSON-shape bug: doctor's caddy_shack-running
	// and caddy-route checks both consume real caddy_shack output, so a
	// drift in that wire format would land here as a FAIL.
	out, err = exec.Command("mailpitter", "doctor", "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("doctor: %v: %s", err, out)
	}
	if !strings.Contains(string(out), `"fail": 0`) {
		t.Errorf("doctor reports non-zero FAILs against a healthy install:\n%s", out)
	}

	// Deregister
	if err := exec.Command("mailpitter", "deregister", "smoke").Run(); err != nil {
		t.Fatalf("deregister: %v", err)
	}

	// Stop: container goes to stopped state.
	if out, err := exec.Command("mailpitter", "stop").CombinedOutput(); err != nil {
		t.Fatalf("stop: %v: %s", err, out)
	}
	out, err = exec.Command("mailpitter", "status", "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("status after stop: %v: %s", err, out)
	}
	if !strings.Contains(string(out), `"container": "stopped"`) {
		t.Errorf("status JSON after stop should report stopped: %s", out)
	}

	// Reset: container and volume removed.
	if out, err := exec.Command("mailpitter", "reset", "--confirm=yes").CombinedOutput(); err != nil {
		t.Fatalf("reset: %v: %s", err, out)
	}
	out, err = exec.Command("mailpitter", "status", "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("status after reset: %v: %s", err, out)
	}
	if !strings.Contains(string(out), `"container": "absent"`) {
		t.Errorf("status JSON after reset should report absent: %s", out)
	}
}

func portReachable(addr string, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}
