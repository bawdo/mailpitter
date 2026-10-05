package docker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// execCommand is a seam over exec.Command so tests can stub the `docker`
// shell-out (see cli_test.go's helper-process pattern). Production always
// uses the real exec.Command.
var execCommand = exec.Command

// CLI is the production Client. Shells out to the `docker` binary on PATH.
type CLI struct{}

// NewCLI returns a CLI client.
func NewCLI() *CLI { return &CLI{} }

func (CLI) DaemonReachable() error {
	if err := execCommand("docker", "info").Run(); err != nil {
		return fmt.Errorf("%w: %v", ErrDaemonUnreachable, err)
	}
	return nil
}

func (CLI) EnsureVolume(name string) error {
	if out, err := execCommand("docker", "volume", "create", name).CombinedOutput(); err != nil {
		return fmt.Errorf("docker volume create %s: %w: %s", name, err, out)
	}
	return nil
}

func (CLI) RemoveVolume(name string) error {
	out, err := execCommand("docker", "volume", "rm", name).CombinedOutput()
	if err != nil {
		if strings.Contains(strings.ToLower(string(out)), "no such volume") {
			return nil
		}
		return fmt.Errorf("docker volume rm %s: %w: %s", name, err, out)
	}
	return nil
}

func (CLI) PullImage(image string) error {
	if out, err := execCommand("docker", "pull", image).CombinedOutput(); err != nil {
		return fmt.Errorf("docker pull %s: %w: %s", image, err, out)
	}
	return nil
}

func (CLI) ImagePresent(image string) (bool, error) {
	err := execCommand("docker", "image", "inspect", image).Run()
	if err == nil {
		return true, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return false, nil
	}
	return false, fmt.Errorf("docker image inspect: %w", err)
}

func (CLI) ImageID(ref string) (string, error) {
	out, err := execCommand("docker", "image", "inspect", "--format", "{{.Id}}", ref).Output()
	if err != nil {
		return "", fmt.Errorf("docker image inspect %s: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (CLI) ContainerImageID(name string) (string, error) {
	out, err := execCommand("docker", "inspect", "--format", "{{.Image}}", name).Output()
	if err != nil {
		return "", fmt.Errorf("docker inspect %s: %w", name, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (CLI) ContainerState(name string) (ContainerState, error) {
	cmd := execCommand("docker", "inspect", "--format", "{{.State.Running}}", name)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && strings.Contains(strings.ToLower(stderr.String()), "no such object") {
			return StateAbsent, nil
		}
		return StateAbsent, fmt.Errorf("docker inspect %s: %w: %s", name, err, stderr.String())
	}
	switch strings.TrimSpace(stdout.String()) {
	case "true":
		return StateRunning, nil
	case "false":
		return StateStopped, nil
	default:
		return StateAbsent, fmt.Errorf("docker inspect %s: unexpected output %q", name, stdout.String())
	}
}

func (CLI) CreateAndStart(spec ContainerSpec) error {
	if out, err := execCommand("docker", RunArgs(spec)...).CombinedOutput(); err != nil {
		return fmt.Errorf("docker run: %w: %s", err, out)
	}
	return nil
}

// RunArgs returns the `docker` argv (minus the binary) CreateAndStart
// executes for spec. Exposed so the generated command can be asserted in
// tests without a running daemon.
func RunArgs(spec ContainerSpec) []string {
	args := []string{"run", "-d", "--name", spec.Name, "--restart", "unless-stopped"}
	for _, e := range spec.Env {
		args = append(args, "-e", e)
	}
	for _, p := range spec.PortMappings {
		args = append(args, "-p",
			fmt.Sprintf("%s:%s:%s", p.HostAddr, strconv.Itoa(p.HostPort), strconv.Itoa(p.ContainerPort)))
	}
	for _, v := range spec.Volumes {
		args = append(args, "-v", fmt.Sprintf("%s:%s", v.VolumeName, v.ContainerPath))
	}
	args = append(args, spec.Image)
	args = append(args, spec.Command...)
	return args
}

func (CLI) Start(name string) error {
	if out, err := execCommand("docker", "start", name).CombinedOutput(); err != nil {
		return fmt.Errorf("docker start %s: %w: %s", name, err, out)
	}
	return nil
}

func (CLI) Stop(name string) error {
	if out, err := execCommand("docker", "stop", name).CombinedOutput(); err != nil {
		return fmt.Errorf("docker stop %s: %w: %s", name, err, out)
	}
	return nil
}

func (CLI) Remove(name string) error {
	out, err := execCommand("docker", "rm", "-f", name).CombinedOutput()
	if err != nil {
		lower := strings.ToLower(string(out))
		if strings.Contains(lower, "no such object") || strings.Contains(lower, "no such container") {
			return nil
		}
		return fmt.Errorf("docker rm -f %s: %w: %s", name, err, out)
	}
	return nil
}

func (CLI) EngineInfo() (EngineInfo, error) {
	out, err := execCommand("docker", "info", "--format", "{{json .}}").Output()
	if err != nil {
		return EngineInfo{}, fmt.Errorf("docker info: %w", err)
	}
	var info EngineInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return EngineInfo{}, fmt.Errorf("docker info: parse: %w", err)
	}
	return info, nil
}

// BridgeGateway returns the IPv4 gateway of the default `bridge` network
// (docker0 on a native Linux engine), as reported by the engine's IPAM.
func (CLI) BridgeGateway() (string, error) {
	out, err := execCommand("docker", "network", "inspect", "bridge", "--format", "{{json .IPAM.Config}}").Output()
	if err != nil {
		return "", fmt.Errorf("docker network inspect bridge: %w", err)
	}
	var cfgs []struct{ Gateway string }
	if err := json.Unmarshal(out, &cfgs); err != nil {
		return "", fmt.Errorf("docker network inspect bridge: parse: %w", err)
	}
	for _, c := range cfgs {
		if ip := net.ParseIP(c.Gateway); ip != nil && ip.To4() != nil {
			return c.Gateway, nil
		}
	}
	return "", fmt.Errorf("docker network inspect bridge: no IPv4 gateway in %s", strings.TrimSpace(string(out)))
}

// ContainerPortBindings returns the host port bindings of an existing
// container, sorted by container port then host address.
func (CLI) ContainerPortBindings(name string) ([]PortMapping, error) {
	out, err := execCommand("docker", "inspect", "--format", "{{json .HostConfig.PortBindings}}", name).Output()
	if err != nil {
		return nil, fmt.Errorf("docker inspect %s: %w", name, err)
	}
	var raw map[string][]struct {
		HostIP   string `json:"HostIp"`
		HostPort string `json:"HostPort"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("docker inspect %s: parse port bindings: %w", name, err)
	}
	var pms []PortMapping
	for key, binds := range raw {
		cport, err := strconv.Atoi(strings.SplitN(key, "/", 2)[0])
		if err != nil {
			return nil, fmt.Errorf("docker inspect %s: bad port key %q", name, key)
		}
		for _, b := range binds {
			hport, _ := strconv.Atoi(b.HostPort)
			pms = append(pms, PortMapping{HostAddr: b.HostIP, HostPort: hport, ContainerPort: cport})
		}
	}
	sort.Slice(pms, func(i, j int) bool {
		if pms[i].ContainerPort != pms[j].ContainerPort {
			return pms[i].ContainerPort < pms[j].ContainerPort
		}
		return pms[i].HostAddr < pms[j].HostAddr
	})
	return pms, nil
}

// ContainerMounts returns the named-volume mounts of an existing container.
// Bind mounts and anonymous tmpfs mounts are omitted.
func (CLI) ContainerMounts(name string) ([]VolumeMount, error) {
	out, err := execCommand("docker", "inspect", "--format", "{{json .Mounts}}", name).Output()
	if err != nil {
		return nil, fmt.Errorf("docker inspect %s: %w", name, err)
	}
	var raw []struct {
		Type        string
		Name        string
		Destination string
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("docker inspect %s: parse mounts: %w", name, err)
	}
	var vms []VolumeMount
	for _, m := range raw {
		if m.Type == "volume" {
			vms = append(vms, VolumeMount{VolumeName: m.Name, ContainerPath: m.Destination})
		}
	}
	return vms, nil
}

// ContainerNetworks returns the names of the networks an existing
// container is attached to, sorted.
func (CLI) ContainerNetworks(name string) ([]string, error) {
	out, err := execCommand("docker", "inspect", "--format", "{{json .NetworkSettings.Networks}}", name).Output()
	if err != nil {
		return nil, fmt.Errorf("docker inspect %s: %w", name, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("docker inspect %s: parse networks: %w", name, err)
	}
	nets := make([]string, 0, len(raw))
	for n := range raw {
		nets = append(nets, n)
	}
	sort.Strings(nets)
	return nets, nil
}

func (CLI) ConnectNetwork(network, container string) error {
	if out, err := execCommand("docker", "network", "connect", network, container).CombinedOutput(); err != nil {
		return fmt.Errorf("docker network connect %s %s: %w: %s", network, container, err, out)
	}
	return nil
}
