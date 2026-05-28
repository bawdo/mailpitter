package docker

import (
	"errors"
	"testing"
)

func TestFakeRecordsCalls(t *testing.T) {
	f := NewFake()
	f.States["mailpitter"] = StateAbsent

	if err := f.DaemonReachable(); err != nil {
		t.Fatalf("DaemonReachable: %v", err)
	}
	if err := f.PullImage("axllent/mailpit:v1.21"); err != nil {
		t.Fatalf("PullImage: %v", err)
	}
	if err := f.EnsureVolume("mailpitter_data"); err != nil {
		t.Fatalf("EnsureVolume: %v", err)
	}
	if got := f.Calls; len(got) != 3 {
		t.Errorf("Calls = %v", got)
	}
	if f.Calls[1] != "PullImage:axllent/mailpit:v1.21" {
		t.Errorf("Calls[1] = %q", f.Calls[1])
	}
}

func TestFakeContainerStateDefaultsAbsent(t *testing.T) {
	f := NewFake()
	st, err := f.ContainerState("missing")
	if err != nil {
		t.Fatalf("ContainerState: %v", err)
	}
	if st != StateAbsent {
		t.Errorf("state = %v", st)
	}
}

func TestFakeErrorsPropagate(t *testing.T) {
	f := NewFake()
	f.DaemonErr = errors.New("daemon down")
	if err := f.DaemonReachable(); err == nil {
		t.Fatal("expected error")
	}
}

func TestFakeRemoveVolume(t *testing.T) {
	f := NewFake()
	f.Volumes["mailpitter_data"] = true
	if err := f.RemoveVolume("mailpitter_data"); err != nil {
		t.Fatalf("RemoveVolume: %v", err)
	}
	if f.Volumes["mailpitter_data"] {
		t.Errorf("volume should be gone")
	}
}
