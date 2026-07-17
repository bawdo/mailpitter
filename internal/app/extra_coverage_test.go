package app

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/bawdo/mailpitter/internal/caddyshack"
	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/exitcode"
)

// brokenLockApp builds an App whose LockPath sits under a regular file, so
// state.AcquireLock fails at MkdirAll. This drives the non-timeout lock
// error branch (return err) without the 5s wait a real contention needs.
func brokenLockApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	p := Paths{
		StateDir:  dir,
		StatePath: filepath.Join(dir, "state.json"),
		LockPath:  filepath.Join(blocker, "lock"), // parent is a file
	}
	return NewWithOptions(Options{
		Docker: docker.NewFake(), Caddy: &fakeCaddy{}, Mailpit: &fakeMailpit{}, Paths: &p,
	})
}

func TestLockAcquisitionErrorBranches(t *testing.T) {
	t.Run("register", func(t *testing.T) {
		a := brokenLockApp(t)
		if _, err := a.Register(context.Background(), RegisterInput{Name: "taiga", Service: "svc"}); err == nil {
			t.Error("expected lock acquisition error")
		}
	})
	t.Run("deregister", func(t *testing.T) {
		a := brokenLockApp(t)
		if _, err := a.Deregister(context.Background(), "taiga"); err == nil {
			t.Error("expected lock acquisition error")
		}
	})
	t.Run("reset", func(t *testing.T) {
		a := brokenLockApp(t)
		if err := a.Reset(context.Background()); err == nil {
			t.Error("expected lock acquisition error")
		}
	})
}

// seedCorrupt writes an unparseable state.json so state.Load returns
// *ErrCorrupt, driving the corrupt-state branches.
func seedCorrupt(t *testing.T, a *App) {
	t.Helper()
	if err := os.WriteFile(a.paths.StatePath, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("seed corrupt state: %v", err)
	}
}

func TestErrorConstructorsWrapSentinels(t *testing.T) {
	cases := []struct {
		got  error
		want error
	}{
		{MailpitNotRunningf("x"), exitcode.ErrMailpitNotRunning},
		{StateCorruptf("x"), exitcode.ErrStateCorrupt},
		{StateLockf("x"), exitcode.ErrStateLockContention},
	}
	for _, tc := range cases {
		if !errors.Is(tc.got, tc.want) {
			t.Errorf("%v should wrap %v", tc.got, tc.want)
		}
	}
}

func TestSeverityString(t *testing.T) {
	cases := map[Severity]string{
		SeverityPass: "PASS",
		SeverityWarn: "WARN",
		SeverityFail: "FAIL",
		SeveritySkip: "SKIP",
		Severity(99): "????",
	}
	for sev, want := range cases {
		if got := sev.String(); got != want {
			t.Errorf("Severity(%d).String() = %q, want %q", sev, got, want)
		}
	}
}

func TestCheckPortFree(t *testing.T) {
	// A port we bind then release is reported free (PASS).
	l, err := net.Listen("tcp", net.JoinHostPort(HostBindAddr, "0"))
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_, portStr, _ := net.SplitHostPort(l.Addr().String())
	freePort, _ := strconv.Atoi(portStr)
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if r := checkPortFree("p", freePort); r.Status != SeverityPass {
		t.Errorf("free port: status = %v, want PASS", r.Status)
	}

	// A held port is reported in use (WARN).
	l2, err := net.Listen("tcp", net.JoinHostPort(HostBindAddr, "0"))
	if err != nil {
		t.Fatalf("listen2: %v", err)
	}
	defer func() { _ = l2.Close() }()
	_, portStr2, _ := net.SplitHostPort(l2.Addr().String())
	usedPort, _ := strconv.Atoi(portStr2)
	if r := checkPortFree("p", usedPort); r.Status != SeverityWarn {
		t.Errorf("in-use port: status = %v, want WARN", r.Status)
	}
}

func TestOpenNonDarwin(t *testing.T) {
	orig := goos
	goos = "linux"
	t.Cleanup(func() { goos = orig })

	var buf bytes.Buffer
	a := NewWithOptions(Options{
		Out: &buf, Docker: docker.NewFake(), Caddy: &fakeCaddy{}, Mailpit: &fakeMailpit{},
	})
	if err := a.Open(context.Background()); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !strings.Contains(buf.String(), "auto-open not supported") {
		t.Errorf("expected manual-open notice, got %q", buf.String())
	}
}

func TestOpenDarwin(t *testing.T) {
	origGoos, origExec := goos, execCommand
	goos = "darwin"
	execCommand = func(_ string, _ ...string) *exec.Cmd { return exec.Command("true") }
	t.Cleanup(func() { goos, execCommand = origGoos, origExec })

	a, _, _ := newTestApp(t)
	if err := a.Open(context.Background()); err != nil {
		t.Fatalf("Open darwin: %v", err)
	}
}

func TestStartErrorBranches(t *testing.T) {
	t.Run("daemon-unreachable", func(t *testing.T) {
		a, fd, _ := newTestApp(t)
		fd.DaemonErr = errors.New("down")
		if err := a.Start(context.Background()); !errors.Is(err, exitcode.ErrDockerUnreachable) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("container-state-error", func(t *testing.T) {
		a, fd, _ := newTestApp(t)
		fd.ContainerStateErr = errors.New("inspect failed")
		if err := a.Start(context.Background()); err == nil {
			t.Error("expected ContainerState error")
		}
	})
	t.Run("docker-start-error", func(t *testing.T) {
		a, fd, _ := newTestApp(t)
		fd.States[ContainerName] = docker.StateStopped
		fd.StartErr = errors.New("start failed")
		if err := a.Start(context.Background()); err == nil {
			t.Error("expected docker Start error")
		}
	})
	t.Run("caddy-register-error", func(t *testing.T) {
		a, fd, fc := newTestApp(t)
		fd.States[ContainerName] = docker.StateRunning
		fc.registerErr = errors.New("caddy down")
		if err := a.Start(context.Background()); !errors.Is(err, exitcode.ErrCaddyShackFailure) {
			t.Errorf("got %v", err)
		}
	})
}

func TestStopErrorBranches(t *testing.T) {
	t.Run("daemon-unreachable", func(t *testing.T) {
		a, fd, _ := newTestApp(t)
		fd.DaemonErr = errors.New("down")
		if err := a.Stop(context.Background()); !errors.Is(err, exitcode.ErrDockerUnreachable) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("container-state-error", func(t *testing.T) {
		a, fd, _ := newTestApp(t)
		fd.ContainerStateErr = errors.New("inspect failed")
		if err := a.Stop(context.Background()); err == nil {
			t.Error("expected ContainerState error")
		}
	})
	t.Run("docker-stop-error", func(t *testing.T) {
		a, fd, _ := newTestApp(t)
		fd.States[ContainerName] = docker.StateRunning
		fd.StopErr = errors.New("stop failed")
		if err := a.Stop(context.Background()); err == nil {
			t.Error("expected docker Stop error")
		}
	})
}

func TestInstallErrorBranches(t *testing.T) {
	t.Run("daemon-unreachable", func(t *testing.T) {
		a, fd, _ := newTestApp(t)
		fd.DaemonErr = errors.New("down")
		if err := a.Install(context.Background(), InstallInput{}); !errors.Is(err, exitcode.ErrDockerUnreachable) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("corrupt-state", func(t *testing.T) {
		a, _, _ := newTestApp(t)
		seedCorrupt(t, a)
		if err := a.Install(context.Background(), InstallInput{}); !errors.Is(err, exitcode.ErrStateCorrupt) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("mode-switch-rejected", func(t *testing.T) {
		a, _, _ := newTestApp(t)
		if err := a.Install(context.Background(), InstallInput{Ephemeral: false}); err != nil {
			t.Fatalf("first install: %v", err)
		}
		err := a.Install(context.Background(), InstallInput{Ephemeral: true})
		if !errors.Is(err, exitcode.ErrInvalidArgs) {
			t.Errorf("mode switch got %v, want ErrInvalidArgs", err)
		}
	})
	t.Run("pull-error", func(t *testing.T) {
		a, fd, _ := newTestApp(t)
		fd.PullImageErr = errors.New("pull failed")
		if err := a.Install(context.Background(), InstallInput{}); err == nil {
			t.Error("expected pull error")
		}
	})
	t.Run("ensure-volume-error", func(t *testing.T) {
		a, fd, _ := newTestApp(t)
		fd.EnsureVolumeErr = errors.New("vol failed")
		if err := a.Install(context.Background(), InstallInput{}); err == nil {
			t.Error("expected volume error")
		}
	})
	t.Run("create-error-triggers-rollback", func(t *testing.T) {
		a, fd, _ := newTestApp(t)
		fd.CreateAndStartErr = errors.New("create failed")
		if err := a.Install(context.Background(), InstallInput{}); err == nil {
			t.Error("expected create error")
		}
		rolledBack := false
		for _, c := range fd.Calls {
			if c == "Remove:"+ContainerName {
				rolledBack = true
			}
		}
		if !rolledBack {
			t.Error("expected rollback Remove after CreateAndStart failure")
		}
	})
	t.Run("start-stopped-error", func(t *testing.T) {
		a, fd, _ := newTestApp(t)
		fd.States[ContainerName] = docker.StateStopped
		fd.StartErr = errors.New("start failed")
		if err := a.Install(context.Background(), InstallInput{}); err == nil {
			t.Error("expected start error")
		}
	})
	t.Run("caddy-not-installed", func(t *testing.T) {
		a, _, fc := newTestApp(t)
		fc.registerErr = caddyshack.ErrNotInstalled
		if err := a.Install(context.Background(), InstallInput{}); !errors.Is(err, exitcode.ErrPrerequisite) {
			t.Errorf("got %v, want ErrPrerequisite", err)
		}
	})
}

func TestRegisterCorruptState(t *testing.T) {
	a, _, _ := newTestApp(t)
	seedCorrupt(t, a)
	if _, err := a.Register(context.Background(), RegisterInput{Name: "taiga", Service: "svc"}); !errors.Is(err, exitcode.ErrStateCorrupt) {
		t.Errorf("got %v", err)
	}
}

func TestDeregisterBranches(t *testing.T) {
	t.Run("corrupt-state", func(t *testing.T) {
		a, _, _ := newTestApp(t)
		seedCorrupt(t, a)
		if _, err := a.Deregister(context.Background(), "taiga"); !errors.Is(err, exitcode.ErrStateCorrupt) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("unknown-name-notice", func(t *testing.T) {
		a, _, _ := newTestApp(t)
		notice, err := a.Deregister(context.Background(), "ghost")
		if err != nil {
			t.Fatalf("Deregister: %v", err)
		}
		if !strings.Contains(notice, "no such consumer") {
			t.Errorf("notice = %q", notice)
		}
	})
}

func TestResetErrorBranches(t *testing.T) {
	t.Run("stop-error", func(t *testing.T) {
		a, fd, _ := newTestApp(t)
		fd.DaemonErr = errors.New("down") // Stop fails at DaemonReachable
		if err := a.Reset(context.Background()); err == nil {
			t.Error("expected Reset to surface Stop error")
		}
	})
	t.Run("remove-error", func(t *testing.T) {
		a, fd, _ := newTestApp(t)
		fd.RemoveErr = errors.New("remove failed")
		if err := a.Reset(context.Background()); err == nil {
			t.Error("expected remove error")
		}
	})
	t.Run("remove-volume-error", func(t *testing.T) {
		a, fd, _ := newTestApp(t)
		fd.RemoveVolumeErr = errors.New("rmvol failed")
		if err := a.Reset(context.Background()); err == nil {
			t.Error("expected remove volume error")
		}
	})
	t.Run("corrupt-state", func(t *testing.T) {
		a, _, _ := newTestApp(t)
		seedCorrupt(t, a)
		if err := a.Reset(context.Background()); !errors.Is(err, exitcode.ErrStateCorrupt) {
			t.Errorf("got %v", err)
		}
	})
}
