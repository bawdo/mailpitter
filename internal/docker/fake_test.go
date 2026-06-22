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

func TestFakeImageIDs(t *testing.T) {
	f := NewFake()
	f.ImageIDs["axllent/mailpit:latest"] = "sha256:aaa"
	f.ContainerImageIDs["mailpitter"] = "sha256:bbb"

	id, err := f.ImageID("axllent/mailpit:latest")
	if err != nil || id != "sha256:aaa" {
		t.Errorf("ImageID = %q, %v", id, err)
	}
	cid, err := f.ContainerImageID("mailpitter")
	if err != nil || cid != "sha256:bbb" {
		t.Errorf("ContainerImageID = %q, %v", cid, err)
	}
	if missing, _ := f.ImageID("nope"); missing != "" {
		t.Errorf("unknown ref should return empty, got %q", missing)
	}
}
