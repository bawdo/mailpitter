package docker

import "fmt"

// Fake is an in-memory Client implementation for unit tests.
type Fake struct {
	DaemonErr error

	// States indexes container name -> state. Missing entries default to absent.
	States map[string]ContainerState

	// Volumes is the set of existing volume names.
	Volumes map[string]bool

	// Images is the set of pulled image references.
	Images map[string]bool

	// Calls records the sequence of method invocations (e.g.
	// "PullImage:axllent/mailpit:v1.21"). Tests assert against it.
	Calls []string

	// LastSpec is the most recent ContainerSpec passed to CreateAndStart.
	LastSpec ContainerSpec
}

// NewFake returns an initialised Fake.
func NewFake() *Fake {
	return &Fake{
		States:  map[string]ContainerState{},
		Volumes: map[string]bool{},
		Images:  map[string]bool{},
	}
}

func (f *Fake) record(s string) { f.Calls = append(f.Calls, s) }

func (f *Fake) DaemonReachable() error {
	f.record("DaemonReachable")
	return f.DaemonErr
}

func (f *Fake) EnsureVolume(name string) error {
	f.record("EnsureVolume:" + name)
	f.Volumes[name] = true
	return nil
}

func (f *Fake) RemoveVolume(name string) error {
	f.record("RemoveVolume:" + name)
	delete(f.Volumes, name)
	return nil
}

func (f *Fake) PullImage(image string) error {
	f.record("PullImage:" + image)
	f.Images[image] = true
	return nil
}

func (f *Fake) ImagePresent(image string) (bool, error) {
	f.record("ImagePresent:" + image)
	return f.Images[image], nil
}

func (f *Fake) ContainerState(name string) (ContainerState, error) {
	f.record("ContainerState:" + name)
	if st, ok := f.States[name]; ok {
		return st, nil
	}
	return StateAbsent, nil
}

func (f *Fake) CreateAndStart(spec ContainerSpec) error {
	f.record(fmt.Sprintf("CreateAndStart:%s", spec.Name))
	f.LastSpec = spec
	f.States[spec.Name] = StateRunning
	return nil
}

func (f *Fake) Start(name string) error {
	f.record("Start:" + name)
	f.States[name] = StateRunning
	return nil
}

func (f *Fake) Stop(name string) error {
	f.record("Stop:" + name)
	f.States[name] = StateStopped
	return nil
}

func (f *Fake) Remove(name string) error {
	f.record("Remove:" + name)
	delete(f.States, name)
	return nil
}
