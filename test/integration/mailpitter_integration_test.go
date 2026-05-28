//go:build integration

package integration

import (
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func have(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func TestMailpitterRoundTrip(t *testing.T) {
	if !have("docker") {
		t.Skip("docker not on PATH")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skipf("docker daemon unreachable: %v", err)
	}
	if !have("caddy_shack") {
		t.Skip("caddy_shack not on PATH")
	}
	if !have("mailpitter") {
		t.Skip("mailpitter not on PATH (run `make install` first)")
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
