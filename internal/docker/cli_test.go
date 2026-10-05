package docker

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// TestHelperProcess is not a real test. It is re-executed as the stubbed
// `docker` binary by fakeExec, emitting canned stdout/stderr and exit code
// from the environment. See https://pkg.go.dev/os/exec#example-Cmd for the
// idiom used by the standard library's own exec tests.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	if s := os.Getenv("HELPER_STDOUT"); s != "" {
		_, _ = os.Stdout.WriteString(s)
	}
	if s := os.Getenv("HELPER_STDERR"); s != "" {
		_, _ = os.Stderr.WriteString(s)
	}
	code, _ := strconv.Atoi(os.Getenv("HELPER_EXIT"))
	os.Exit(code)
}

// fakeExec installs an execCommand stub that re-runs the test binary as the
// helper process, then restores the original via t.Cleanup.
func fakeExec(t *testing.T, stdout, stderr string, exit int) {
	t.Helper()
	orig := execCommand
	execCommand = func(_ string, _ ...string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
		cmd.Env = []string{
			"GO_WANT_HELPER_PROCESS=1",
			"HELPER_STDOUT=" + stdout,
			"HELPER_STDERR=" + stderr,
			"HELPER_EXIT=" + strconv.Itoa(exit),
		}
		return cmd
	}
	t.Cleanup(func() { execCommand = orig })
}

// failExec installs a stub that points at a binary that does not exist, so
// the command fails to start (a non-ExitError). This exercises the branches
// that distinguish "ran and exited non-zero" from "could not run at all".
func failExec(t *testing.T) {
	t.Helper()
	orig := execCommand
	execCommand = func(_ string, _ ...string) *exec.Cmd {
		return exec.Command("/nonexistent/mailpitter-docker-stub")
	}
	t.Cleanup(func() { execCommand = orig })
}

func TestDaemonReachable(t *testing.T) {
	fakeExec(t, "", "", 0)
	if err := (CLI{}).DaemonReachable(); err != nil {
		t.Fatalf("DaemonReachable: %v", err)
	}
	fakeExec(t, "", "cannot connect", 1)
	if err := (CLI{}).DaemonReachable(); err == nil {
		t.Fatal("expected error when daemon unreachable")
	}
}

func TestEnsureVolume(t *testing.T) {
	fakeExec(t, "", "", 0)
	if err := (CLI{}).EnsureVolume("vol"); err != nil {
		t.Fatalf("EnsureVolume: %v", err)
	}
	fakeExec(t, "", "boom", 1)
	if err := (CLI{}).EnsureVolume("vol"); err == nil {
		t.Fatal("expected error")
	}
}

func TestRemoveVolume(t *testing.T) {
	fakeExec(t, "", "", 0)
	if err := (CLI{}).RemoveVolume("vol"); err != nil {
		t.Fatalf("RemoveVolume: %v", err)
	}
	// "no such volume" is swallowed as success (idempotent removal).
	fakeExec(t, "", "Error: No such volume: vol", 1)
	if err := (CLI{}).RemoveVolume("vol"); err != nil {
		t.Fatalf("RemoveVolume swallow: %v", err)
	}
	fakeExec(t, "", "some other failure", 1)
	if err := (CLI{}).RemoveVolume("vol"); err == nil {
		t.Fatal("expected error for non-missing failure")
	}
}

func TestPullImage(t *testing.T) {
	fakeExec(t, "", "", 0)
	if err := (CLI{}).PullImage("img"); err != nil {
		t.Fatalf("PullImage: %v", err)
	}
	fakeExec(t, "", "not found", 1)
	if err := (CLI{}).PullImage("img"); err == nil {
		t.Fatal("expected error")
	}
}

func TestImagePresent(t *testing.T) {
	fakeExec(t, "", "", 0)
	present, err := (CLI{}).ImagePresent("img")
	if err != nil || !present {
		t.Fatalf("present=%v err=%v; want true,nil", present, err)
	}
	// Ran and exited non-zero => image absent, no error.
	fakeExec(t, "", "No such image", 1)
	present, err = (CLI{}).ImagePresent("img")
	if err != nil || present {
		t.Fatalf("present=%v err=%v; want false,nil", present, err)
	}
	// Could not run at all => surfaced as an error.
	failExec(t)
	if _, err := (CLI{}).ImagePresent("img"); err == nil {
		t.Fatal("expected error when docker cannot run")
	}
}

func TestImageID(t *testing.T) {
	fakeExec(t, "sha256:abc\n", "", 0)
	id, err := (CLI{}).ImageID("img")
	if err != nil {
		t.Fatalf("ImageID: %v", err)
	}
	if id != "sha256:abc" {
		t.Errorf("id = %q, want trimmed sha256:abc", id)
	}
	fakeExec(t, "", "no such image", 1)
	if _, err := (CLI{}).ImageID("img"); err == nil {
		t.Fatal("expected error")
	}
}

func TestContainerImageID(t *testing.T) {
	fakeExec(t, "sha256:def\n", "", 0)
	id, err := (CLI{}).ContainerImageID("c")
	if err != nil {
		t.Fatalf("ContainerImageID: %v", err)
	}
	if id != "sha256:def" {
		t.Errorf("id = %q", id)
	}
	fakeExec(t, "", "no such object", 1)
	if _, err := (CLI{}).ContainerImageID("c"); err == nil {
		t.Fatal("expected error")
	}
}

func TestContainerState(t *testing.T) {
	cases := []struct {
		name   string
		stdout string
		stderr string
		exit   int
		want   ContainerState
		errX   bool
	}{
		{"running", "true\n", "", 0, StateRunning, false},
		{"stopped", "false\n", "", 0, StateStopped, false},
		{"absent-no-such-object", "", "Error: No such object: c", 1, StateAbsent, false},
		{"unexpected-output", "maybe\n", "", 0, StateAbsent, true},
		{"other-failure", "", "daemon down", 1, StateAbsent, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeExec(t, tc.stdout, tc.stderr, tc.exit)
			st, err := (CLI{}).ContainerState("c")
			if tc.errX && err == nil {
				t.Fatal("expected error")
			}
			if !tc.errX && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if st != tc.want {
				t.Errorf("state = %v, want %v", st, tc.want)
			}
		})
	}
}

func TestCreateAndStart(t *testing.T) {
	spec := ContainerSpec{
		Name:         "mailpitter",
		Image:        "img",
		Env:          []string{"K=V"},
		PortMappings: []PortMapping{{HostAddr: "127.0.0.1", HostPort: 8025, ContainerPort: 8025}},
		Volumes:      []VolumeMount{{VolumeName: "vol", ContainerPath: "/data"}},
		Command:      []string{"--flag"},
	}
	fakeExec(t, "", "", 0)
	if err := (CLI{}).CreateAndStart(spec); err != nil {
		t.Fatalf("CreateAndStart: %v", err)
	}
	fakeExec(t, "", "port already allocated", 1)
	if err := (CLI{}).CreateAndStart(spec); err == nil {
		t.Fatal("expected error")
	}
}

func TestStartStop(t *testing.T) {
	fakeExec(t, "", "", 0)
	if err := (CLI{}).Start("c"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	fakeExec(t, "", "boom", 1)
	if err := (CLI{}).Start("c"); err == nil {
		t.Fatal("expected Start error")
	}
	fakeExec(t, "", "", 0)
	if err := (CLI{}).Stop("c"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	fakeExec(t, "", "boom", 1)
	if err := (CLI{}).Stop("c"); err == nil {
		t.Fatal("expected Stop error")
	}
}

func TestRemove(t *testing.T) {
	fakeExec(t, "", "", 0)
	if err := (CLI{}).Remove("c"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	for _, msg := range []string{"Error: No such object: c", "Error: No such container: c"} {
		fakeExec(t, "", msg, 1)
		if err := (CLI{}).Remove("c"); err != nil {
			t.Fatalf("Remove should swallow %q: %v", msg, err)
		}
	}
	fakeExec(t, "", "container is paused", 1)
	if err := (CLI{}).Remove("c"); err == nil {
		t.Fatal("expected error for non-missing failure")
	}
}

// Sanity guard: the stubbed helper never leaks the real docker binary path.
func TestFakeExecIsolation(t *testing.T) {
	fakeExec(t, "hi", "", 0)
	out, err := execCommand("docker", "anything").Output()
	if err != nil {
		t.Fatalf("helper: %v", err)
	}
	if !strings.Contains(string(out), "hi") {
		t.Errorf("out = %q", out)
	}
}

func TestRunArgs(t *testing.T) {
	spec := ContainerSpec{
		Name:  "c",
		Image: "img",
		Env:   []string{"A=1"},
		PortMappings: []PortMapping{
			{HostAddr: "127.0.0.1", HostPort: 1025, ContainerPort: 1025},
			{HostAddr: "172.17.0.1", HostPort: 1025, ContainerPort: 1025},
		},
		Volumes: []VolumeMount{{VolumeName: "v", ContainerPath: "/data"}},
		Command: []string{"--flag"},
	}
	got := strings.Join(RunArgs(spec), " ")
	want := "run -d --name c --restart unless-stopped -e A=1 -p 127.0.0.1:1025:1025 -p 172.17.0.1:1025:1025 -v v:/data img --flag"
	if got != want {
		t.Errorf("RunArgs =\n %s\nwant\n %s", got, want)
	}
}

func TestEngineInfo(t *testing.T) {
	fakeExec(t, `{"OperatingSystem":"Docker Desktop","OSType":"linux","Name":"docker-desktop","Labels":["com.docker.desktop.address=x"],"Other":1}`, "", 0)
	info, err := (CLI{}).EngineInfo()
	if err != nil {
		t.Fatalf("EngineInfo: %v", err)
	}
	if info.OperatingSystem != "Docker Desktop" || info.OSType != "linux" || info.Name != "docker-desktop" || len(info.Labels) != 1 {
		t.Errorf("info = %+v", info)
	}
	fakeExec(t, "not json", "", 0)
	if _, err := (CLI{}).EngineInfo(); err == nil {
		t.Error("expected parse error")
	}
	fakeExec(t, "", "boom", 1)
	if _, err := (CLI{}).EngineInfo(); err == nil {
		t.Error("expected exec error")
	}
}

func TestBridgeGateway(t *testing.T) {
	fakeExec(t, `[{"Subnet":"fd00::/64","Gateway":"fd00::1"},{"Subnet":"172.17.0.0/16","Gateway":"172.17.0.1"}]`, "", 0)
	gw, err := (CLI{}).BridgeGateway()
	if err != nil || gw != "172.17.0.1" {
		t.Fatalf("BridgeGateway = %q, %v", gw, err)
	}
	for _, out := range []string{`[]`, `[{"Subnet":"172.17.0.0/16"}]`, `null`, `bad`} {
		fakeExec(t, out, "", 0)
		if gw, err := (CLI{}).BridgeGateway(); err == nil {
			t.Errorf("%s: expected error, got %q", out, gw)
		}
	}
	fakeExec(t, "", "no such network", 1)
	if _, err := (CLI{}).BridgeGateway(); err == nil {
		t.Error("expected exec error")
	}
}

func TestContainerPortBindings(t *testing.T) {
	fakeExec(t, `{"8025/tcp":[{"HostIp":"127.0.0.1","HostPort":"8025"}],"1025/tcp":[{"HostIp":"172.17.0.1","HostPort":"1025"},{"HostIp":"127.0.0.1","HostPort":"1025"}]}`, "", 0)
	got, err := (CLI{}).ContainerPortBindings("c")
	if err != nil {
		t.Fatalf("ContainerPortBindings: %v", err)
	}
	want := []PortMapping{
		{HostAddr: "127.0.0.1", HostPort: 1025, ContainerPort: 1025},
		{HostAddr: "172.17.0.1", HostPort: 1025, ContainerPort: 1025},
		{HostAddr: "127.0.0.1", HostPort: 8025, ContainerPort: 8025},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	fakeExec(t, `null`, "", 0)
	if got, err := (CLI{}).ContainerPortBindings("c"); err != nil || len(got) != 0 {
		t.Errorf("null bindings = %+v, %v", got, err)
	}
	for _, out := range []string{`bad`, `{"x/tcp":[]}`} {
		fakeExec(t, out, "", 0)
		if _, err := (CLI{}).ContainerPortBindings("c"); err == nil {
			t.Errorf("%s: expected error", out)
		}
	}
	fakeExec(t, "", "no such object", 1)
	if _, err := (CLI{}).ContainerPortBindings("c"); err == nil {
		t.Error("expected exec error")
	}
}

func TestContainerMounts(t *testing.T) {
	fakeExec(t, `[{"Type":"volume","Name":"mailpitter_data","Destination":"/data"},{"Type":"bind","Source":"/x","Destination":"/y"}]`, "", 0)
	got, err := (CLI{}).ContainerMounts("c")
	if err != nil {
		t.Fatalf("ContainerMounts: %v", err)
	}
	if len(got) != 1 || got[0] != (VolumeMount{VolumeName: "mailpitter_data", ContainerPath: "/data"}) {
		t.Errorf("mounts = %+v", got)
	}
	fakeExec(t, `bad`, "", 0)
	if _, err := (CLI{}).ContainerMounts("c"); err == nil {
		t.Error("expected parse error")
	}
	fakeExec(t, "", "no such object", 1)
	if _, err := (CLI{}).ContainerMounts("c"); err == nil {
		t.Error("expected exec error")
	}
}

func TestContainerNetworks(t *testing.T) {
	fakeExec(t, `{"caddy_shack":{"IPAddress":"172.18.0.2"},"bridge":{"IPAddress":"172.17.0.3"}}`, "", 0)
	got, err := (CLI{}).ContainerNetworks("c")
	if err != nil || strings.Join(got, ",") != "bridge,caddy_shack" {
		t.Fatalf("ContainerNetworks = %v, %v", got, err)
	}
	fakeExec(t, `bad`, "", 0)
	if _, err := (CLI{}).ContainerNetworks("c"); err == nil {
		t.Error("expected parse error")
	}
	fakeExec(t, "", "no such object", 1)
	if _, err := (CLI{}).ContainerNetworks("c"); err == nil {
		t.Error("expected exec error")
	}
}

func TestConnectNetwork(t *testing.T) {
	fakeExec(t, "", "", 0)
	if err := (CLI{}).ConnectNetwork("n", "c"); err != nil {
		t.Fatalf("ConnectNetwork: %v", err)
	}
	fakeExec(t, "", "boom", 1)
	if err := (CLI{}).ConnectNetwork("n", "c"); err == nil {
		t.Error("expected error")
	}
}
