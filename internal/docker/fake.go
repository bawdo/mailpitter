package docker

import "fmt"

// Fake is an in-memory Client implementation for unit tests.
type Fake struct {
	DaemonErr error

	// Per-method error hooks. When set, the corresponding method returns
	// the error instead of its success result, so callers can exercise
	// their failure branches. All default nil (success), matching the
	// DaemonErr convention above.
	ContainerStateErr error
	EnsureVolumeErr   error
	RemoveVolumeErr   error
	PullImageErr      error
	CreateAndStartErr error
	StartErr          error
	StopErr           error
	RemoveErr         error

	// States indexes container name -> state. Missing entries default to absent.
	States map[string]ContainerState

	// Volumes is the set of existing volume names.
	Volumes map[string]bool

	// Images is the set of pulled image references.
	Images map[string]bool

	// ImageIDs maps an image ref to the ID ImageID returns. Missing -> "".
	ImageIDs map[string]string

	// ContainerImageIDs maps a container name to the ID
	// ContainerImageID returns. Missing -> "".
	ContainerImageIDs map[string]string

	// Calls records the sequence of method invocations (e.g.
	// "PullImage:axllent/mailpit:v1.21"). Tests assert against it.
	Calls []string

	// LastSpec is the most recent ContainerSpec passed to CreateAndStart.
	LastSpec ContainerSpec
}

// NewFake returns an initialised Fake.
func NewFake() *Fake {
	return &Fake{
		States:            map[string]ContainerState{},
		Volumes:           map[string]bool{},
		Images:            map[string]bool{},
		ImageIDs:          map[string]string{},
		ContainerImageIDs: map[string]string{},
	}
}

func (f *Fake) record(s string) { f.Calls = append(f.Calls, s) }

func (f *Fake) DaemonReachable() error {
	f.record("DaemonReachable")
	return f.DaemonErr
}

func (f *Fake) EnsureVolume(name string) error {
	f.record("EnsureVolume:" + name)
	if f.EnsureVolumeErr != nil {
		return f.EnsureVolumeErr
	}
	f.Volumes[name] = true
	return nil
}

func (f *Fake) RemoveVolume(name string) error {
	f.record("RemoveVolume:" + name)
	if f.RemoveVolumeErr != nil {
		return f.RemoveVolumeErr
	}
	delete(f.Volumes, name)
	return nil
}

func (f *Fake) PullImage(image string) error {
	f.record("PullImage:" + image)
	if f.PullImageErr != nil {
		return f.PullImageErr
	}
	f.Images[image] = true
	return nil
}

func (f *Fake) ImagePresent(image string) (bool, error) {
	f.record("ImagePresent:" + image)
	return f.Images[image], nil
}

func (f *Fake) ImageID(ref string) (string, error) {
	f.record("ImageID:" + ref)
	return f.ImageIDs[ref], nil
}

func (f *Fake) ContainerImageID(name string) (string, error) {
	f.record("ContainerImageID:" + name)
	return f.ContainerImageIDs[name], nil
}

func (f *Fake) ContainerState(name string) (ContainerState, error) {
	f.record("ContainerState:" + name)
	if f.ContainerStateErr != nil {
		return StateAbsent, f.ContainerStateErr
	}
	if st, ok := f.States[name]; ok {
		return st, nil
	}
	return StateAbsent, nil
}

func (f *Fake) CreateAndStart(spec ContainerSpec) error {
	f.record(fmt.Sprintf("CreateAndStart:%s", spec.Name))
	if f.CreateAndStartErr != nil {
		return f.CreateAndStartErr
	}
	f.LastSpec = spec
	f.States[spec.Name] = StateRunning
	return nil
}

func (f *Fake) Start(name string) error {
	f.record("Start:" + name)
	if f.StartErr != nil {
		return f.StartErr
	}
	f.States[name] = StateRunning
	return nil
}

func (f *Fake) Stop(name string) error {
	f.record("Stop:" + name)
	if f.StopErr != nil {
		return f.StopErr
	}
	f.States[name] = StateStopped
	return nil
}

func (f *Fake) Remove(name string) error {
	f.record("Remove:" + name)
	if f.RemoveErr != nil {
		return f.RemoveErr
	}
	delete(f.States, name)
	return nil
}
