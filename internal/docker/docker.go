// Package docker is a thin abstraction over the docker CLI for the single
// container mailpitter manages. The interface keeps app-layer tests off
// real docker; the concrete impl in cli.go shells out for production use.
package docker

import "errors"

// ContainerState is the lifecycle phase of mailpitter's mailpit container.
type ContainerState int

const (
	StateAbsent ContainerState = iota
	StateStopped
	StateRunning
)

func (s ContainerState) String() string {
	switch s {
	case StateAbsent:
		return "absent"
	case StateStopped:
		return "stopped"
	case StateRunning:
		return "running"
	default:
		return "unknown"
	}
}

// ErrDaemonUnreachable indicates the docker CLI could not contact the
// daemon.
var ErrDaemonUnreachable = errors.New("docker: daemon unreachable")

// Client is the interface the app layer depends on.
type Client interface {
	DaemonReachable() error
	EnsureVolume(name string) error
	RemoveVolume(name string) error
	PullImage(image string) error
	ImagePresent(image string) (bool, error)
	ImageID(ref string) (string, error)
	ContainerImageID(name string) (string, error)
	ContainerState(name string) (ContainerState, error)
	CreateAndStart(spec ContainerSpec) error
	Start(name string) error
	Stop(name string) error
	Remove(name string) error
	EngineInfo() (EngineInfo, error)
	BridgeGateway() (string, error)
	ContainerPortBindings(name string) ([]PortMapping, error)
	ContainerMounts(name string) ([]VolumeMount, error)
	ContainerNetworks(name string) ([]string, error)
	ConnectNetwork(network, container string) error
}

// EngineInfo is the subset of `docker info` used to classify the engine
// (native Linux Engine vs Docker Desktop / a VM-backed runtime).
type EngineInfo struct {
	OperatingSystem string
	OSType          string
	Name            string
	Labels          []string
}

// ContainerSpec describes the container CreateAndStart should run.
type ContainerSpec struct {
	Name         string
	Image        string
	Env          []string
	PortMappings []PortMapping
	Volumes      []VolumeMount
	Command      []string
}

// PortMapping publishes a container port to a specific host address/port.
type PortMapping struct {
	HostAddr      string
	HostPort      int
	ContainerPort int
}

// VolumeMount mounts a named volume at a container path.
type VolumeMount struct {
	VolumeName    string
	ContainerPath string
}
