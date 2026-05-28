package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/exitcode"
	"github.com/bawdo/mailpitter/internal/mailpitapi"
	"github.com/bawdo/mailpitter/internal/state"
)

func newTestApp(t *testing.T) (*App, *docker.Fake, *fakeCaddy) {
	t.Helper()
	dir := t.TempDir()
	p := Paths{
		StateDir:  dir,
		StatePath: filepath.Join(dir, "state.json"),
		LockPath:  filepath.Join(dir, "state.json.lock"),
	}
	fd := docker.NewFake()
	fc := &fakeCaddy{}
	a := NewWithOptions(Options{Docker: fd, Caddy: fc, Mailpit: &fakeMailpit{}, Paths: &p})
	return a, fd, fc
}

type fakeCaddy struct {
	registers   []string
	deregisters []string
	registerErr error
}

func (f *fakeCaddy) Register(_ context.Context, name, domain string, port int) error {
	f.registers = append(f.registers, name+"|"+domain)
	return f.registerErr
}

func (f *fakeCaddy) Deregister(_ context.Context, name string) error {
	f.deregisters = append(f.deregisters, name)
	return nil
}

type fakeMailpit struct{}

func (fakeMailpit) Info() (mailpitapi.Info, error) { panic("not used in install tests") }

func TestInstallFreshPersistent(t *testing.T) {
	a, fd, fc := newTestApp(t)
	if err := a.Install(context.Background(), InstallInput{Ephemeral: false}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !fd.Volumes["mailpitter_data"] {
		t.Errorf("data volume not created")
	}
	if fd.LastSpec.Name != "mailpitter" {
		t.Errorf("container spec name = %q", fd.LastSpec.Name)
	}
	if len(fd.LastSpec.Volumes) != 1 {
		t.Errorf("persistent install should mount one volume, got %d", len(fd.LastSpec.Volumes))
	}
	if len(fc.registers) != 1 || fc.registers[0] != "mailpit|mailpit.test" {
		t.Errorf("caddy_shack register call = %v", fc.registers)
	}
	// Round-trip: load from app paths.
	loaded, err := state.Load(a.paths.StatePath)
	if err != nil {
		t.Fatalf("Load state: %v", err)
	}
	if loaded.Install == nil || loaded.Install.Mode != state.ModePersistent {
		t.Errorf("install state = %+v", loaded.Install)
	}
}

func TestInstallEphemeralSkipsVolume(t *testing.T) {
	a, fd, _ := newTestApp(t)
	if err := a.Install(context.Background(), InstallInput{Ephemeral: true}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if fd.Volumes["mailpitter_data"] {
		t.Errorf("ephemeral install should not create volume")
	}
	if len(fd.LastSpec.Volumes) != 0 {
		t.Errorf("ephemeral install should mount no volumes, got %d", len(fd.LastSpec.Volumes))
	}
}

func TestInstallIdempotentWhenRunning(t *testing.T) {
	a, fd, fc := newTestApp(t)
	if err := a.Install(context.Background(), InstallInput{Ephemeral: false}); err != nil {
		t.Fatalf("first Install: %v", err)
	}
	// Reset call log for the second invocation.
	fd.Calls = nil
	fc.registers = nil
	if err := a.Install(context.Background(), InstallInput{Ephemeral: false}); err != nil {
		t.Fatalf("second Install: %v", err)
	}
	for _, c := range fd.Calls {
		if c == "CreateAndStart:mailpitter" {
			t.Errorf("second Install should not re-create container; calls=%v", fd.Calls)
		}
	}
	// caddy_shack register is idempotent so we re-call it every time; assert that.
	if len(fc.registers) != 1 {
		t.Errorf("caddy_shack register should be re-called on second install, got %d", len(fc.registers))
	}
}

func TestInstallRefusesModeSwitch(t *testing.T) {
	a, _, _ := newTestApp(t)
	if err := a.Install(context.Background(), InstallInput{Ephemeral: false}); err != nil {
		t.Fatalf("first: %v", err)
	}
	err := a.Install(context.Background(), InstallInput{Ephemeral: true})
	if !errors.Is(err, exitcode.ErrInvalidArgs) {
		t.Errorf("expected ErrInvalidArgs, got %v", err)
	}
}

func TestInstallDockerDown(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.DaemonErr = errors.New("daemon down")
	err := a.Install(context.Background(), InstallInput{})
	if !errors.Is(err, exitcode.ErrDockerUnreachable) {
		t.Errorf("expected ErrDockerUnreachable, got %v", err)
	}
}
