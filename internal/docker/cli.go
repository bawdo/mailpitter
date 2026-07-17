package docker

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
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
	if out, err := execCommand("docker", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("docker run: %w: %s", err, out)
	}
	return nil
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
