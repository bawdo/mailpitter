package app

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/state"
)

const testGateway = "172.17.0.1"

var (
	nativeEngine  = docker.EngineInfo{OperatingSystem: "Omarchy", OSType: "linux", Name: "worklappy"}
	desktopEngine = docker.EngineInfo{
		OperatingSystem: "Docker Desktop", OSType: "linux", Name: "docker-desktop",
		Labels: []string{"com.docker.desktop.address=unix:///Users/x/Library/Containers/com.docker.docker/Data/docker-cli.sock"},
	}
	hostIPs = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("192.168.1.20"), net.ParseIP(testGateway)}
)

// stubHost points the interface and ufw seams at test fixtures. ufwConf
// and ufwRules are written to a temp dir; pass "" to leave a file absent.
func stubHost(t *testing.T, ips []net.IP, ufwConf, ufwRules string) {
	t.Helper()
	origIfaces, origUFW := interfaceAddrs, ufwDir
	interfaceAddrs = func() ([]net.IP, error) { return ips, nil }
	ufwDir = t.TempDir()
	if ufwConf != "" {
		if err := os.WriteFile(filepath.Join(ufwDir, "ufw.conf"), []byte(ufwConf), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if ufwRules != "" {
		if err := os.WriteFile(filepath.Join(ufwDir, "user.rules"), []byte(ufwRules), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { interfaceAddrs, ufwDir = origIfaces, origUFW })
}

// newNativeApp is newTestAppWithNotices on a native Linux engine with ufw
// absent.
func newNativeApp(t *testing.T) (*App, *docker.Fake, *fakeCaddy, noticeBuf) {
	t.Helper()
	a, fd, fc, notices := newTestAppWithNotices(t)
	fd.Engine = nativeEngine
	fd.Gateway = testGateway
	stubHost(t, hostIPs, "", "")
	return a, fd, fc, notices
}

func TestClassifyEngine(t *testing.T) {
	cases := []struct {
		name string
		info docker.EngineInfo
		gw   string
		ips  []net.IP
		want EngineKind
	}{
		{"native linux", nativeEngine, testGateway, hostIPs, EngineNativeLinux},
		{"docker desktop mac", desktopEngine, "172.17.0.1", nil, EngineLoopbackForwarded},
		// Desktop on Linux still runs the engine in a VM; never bind the gateway.
		{"docker desktop linux", docker.EngineInfo{OperatingSystem: "Docker Desktop", OSType: "linux", Name: "docker-desktop"}, testGateway, hostIPs, EngineLoopbackForwarded},
		{"desktop by label only", docker.EngineInfo{OSType: "linux", Labels: []string{"com.docker.desktop.address=x"}}, testGateway, hostIPs, EngineLoopbackForwarded},
		{"orbstack", docker.EngineInfo{OperatingSystem: "OrbStack", OSType: "linux", Name: "orbstack"}, testGateway, nil, EngineLoopbackForwarded},
		{"colima reports plain ubuntu", docker.EngineInfo{OperatingSystem: "Ubuntu 24.04.1 LTS", OSType: "linux", Name: "colima"}, testGateway, nil, EngineLoopbackForwarded},
		{"colima profile", docker.EngineInfo{OperatingSystem: "Ubuntu", OSType: "linux", Name: "colima-work"}, testGateway, nil, EngineLoopbackForwarded},
		{"rancher desktop", docker.EngineInfo{OperatingSystem: "Rancher Desktop WSL Distribution", OSType: "linux", Name: "x"}, testGateway, nil, EngineLoopbackForwarded},
		{"lima", docker.EngineInfo{OperatingSystem: "Alpine Linux", OSType: "linux", Name: "lima-rancher-desktop"}, testGateway, nil, EngineLoopbackForwarded},
		{"unmarked VM: gateway not local", docker.EngineInfo{OperatingSystem: "Ubuntu", OSType: "linux", Name: "box"}, testGateway, []net.IP{net.ParseIP("127.0.0.1")}, EngineUnknown},
		{"windows engine", docker.EngineInfo{OperatingSystem: "Windows Server", OSType: "windows"}, testGateway, hostIPs, EngineUnknown},
		{"empty info", docker.EngineInfo{}, testGateway, hostIPs, EngineUnknown},
		{"ipv6 gateway", nativeEngine, "fd00::1", []net.IP{net.ParseIP("fd00::1")}, EngineUnknown},
		{"garbage gateway", nativeEngine, "nope", hostIPs, EngineUnknown},
		{"loopback gateway", nativeEngine, "127.0.0.1", hostIPs, EngineUnknown},
		{"unspecified gateway", nativeEngine, "0.0.0.0", []net.IP{net.IPv4zero}, EngineUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hn := classifyEngine(tc.info, tc.gw, tc.ips)
			if hn.Kind != tc.want {
				t.Fatalf("kind = %v (%s), want %v", hn.Kind, hn.Reason, tc.want)
			}
			if hn.Reason == "" {
				t.Error("reason should always be set")
			}
			if (hn.Gateway != "") != (tc.want == EngineNativeLinux) {
				t.Errorf("gateway = %q; only native linux should carry one", hn.Gateway)
			}
		})
	}
}

func TestEngineKindString(t *testing.T) {
	for k, want := range map[EngineKind]string{
		EngineUnknown: "unknown", EngineLoopbackForwarded: "desktop-or-vm", EngineNativeLinux: "native-linux",
	} {
		if k.String() != want {
			t.Errorf("%d.String() = %q, want %q", k, k.String(), want)
		}
	}
}

func TestHostNetworkFallsBackToUnknown(t *testing.T) {
	cases := map[string]func(fd *docker.Fake){
		"info error":    func(fd *docker.Fake) { fd.EngineInfoErr = errors.New("boom") },
		"gateway error": func(fd *docker.Fake) { fd.Engine = nativeEngine; fd.GatewayErr = errors.New("no bridge") },
		"iface error": func(fd *docker.Fake) {
			fd.Engine, fd.Gateway = nativeEngine, testGateway
			interfaceAddrs = func() ([]net.IP, error) { return nil, errors.New("no ifaces") }
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			a, fd, _ := newTestApp(t)
			stubHost(t, hostIPs, "", "")
			setup(fd)
			if hn := a.hostNetwork(); hn.Kind != EngineUnknown || hn.Reason == "" {
				t.Errorf("hostNetwork = %+v", hn)
			}
		})
	}
}

func TestHostNetworkDesktopSkipsGatewayLookup(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.Engine = desktopEngine
	fd.GatewayErr = errors.New("must not be needed")
	if hn := a.hostNetwork(); hn.Kind != EngineLoopbackForwarded {
		t.Errorf("hostNetwork = %+v", hn)
	}
}

// TestContainerSpecDesktopArgsUnchanged pins the docker run argv for the
// Docker Desktop (macOS) path to exactly what mailpitter generated before
// the gateway binding existed.
func TestContainerSpecDesktopArgsUnchanged(t *testing.T) {
	cases := map[state.Mode]string{
		state.ModePersistent: "run -d --name mailpitter --restart unless-stopped -e MP_DATABASE=/data/mailpit.db " +
			"-p 127.0.0.1:1025:1025 -p 127.0.0.1:8025:8025 -v mailpitter_data:/data axllent/mailpit:v1.21",
		state.ModeEphemeral: "run -d --name mailpitter --restart unless-stopped " +
			"-p 127.0.0.1:1025:1025 -p 127.0.0.1:8025:8025 axllent/mailpit:v1.21",
	}
	for mode, want := range cases {
		hn := classifyEngine(desktopEngine, testGateway, hostIPs)
		got := strings.Join(docker.RunArgs(containerSpec(mode, Image, hn.Gateway)), " ")
		if got != want {
			t.Errorf("%s:\n got  %s\n want %s", mode, got, want)
		}
	}
}

func TestContainerSpecNativeLinuxArgs(t *testing.T) {
	hn := classifyEngine(nativeEngine, testGateway, hostIPs)
	got := strings.Join(docker.RunArgs(containerSpec(state.ModePersistent, Image, hn.Gateway)), " ")
	want := "run -d --name mailpitter --restart unless-stopped -e MP_DATABASE=/data/mailpit.db " +
		"-p 127.0.0.1:1025:1025 -p 172.17.0.1:1025:1025 -p 127.0.0.1:8025:8025 -v mailpitter_data:/data axllent/mailpit:v1.21"
	if got != want {
		t.Errorf("\n got  %s\n want %s", got, want)
	}
}

// TestContainerSpecNeverBindsWildcard guards the "loopback plus docker0
// gateway only" constraint, and keeps the UI on loopback.
func TestContainerSpecNeverBindsWildcard(t *testing.T) {
	for _, gw := range []string{"", testGateway} {
		for _, mode := range []state.Mode{state.ModePersistent, state.ModeEphemeral} {
			for _, p := range containerSpec(mode, Image, gw).PortMappings {
				if p.HostAddr != HostBindAddr && (p.HostAddr != gw || p.ContainerPort != SMTPPort) {
					t.Errorf("gw=%q mode=%s: unexpected binding %+v", gw, mode, p)
				}
			}
		}
	}
}

func TestInstallFreshNativeLinuxPublishesGateway(t *testing.T) {
	a, fd, _, _ := newNativeApp(t)
	if err := a.Install(context.Background(), InstallInput{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if smtpGatewayMissing(a.hostNetwork(), fd.LastSpec.PortMappings) {
		t.Errorf("fresh native install lacks gateway binding: %+v", fd.LastSpec.PortMappings)
	}
}

func TestInstallFreshUnknownEngineKeepsLoopbackAndSaysSo(t *testing.T) {
	a, fd, _, notices := newTestAppWithNotices(t)
	fd.Engine = docker.EngineInfo{OSType: "linux", OperatingSystem: "Ubuntu"}
	fd.Gateway = testGateway
	stubHost(t, []net.IP{net.ParseIP("127.0.0.1")}, "", "")
	if err := a.Install(context.Background(), InstallInput{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(fd.LastSpec.PortMappings) != 2 {
		t.Errorf("unknown engine should keep loopback-only bindings, got %+v", fd.LastSpec.PortMappings)
	}
	if !strings.Contains(notices.String(), "could not classify the docker engine") {
		t.Errorf("missing unknown-engine notice: %q", notices.String())
	}
}

func TestInstallDesktopIsQuiet(t *testing.T) {
	a, fd, _, notices := newTestAppWithNotices(t)
	fd.Engine = desktopEngine
	if err := a.Install(context.Background(), InstallInput{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if got := notices.String(); got != "registered mailpit.test with caddy_shack\n" {
		t.Errorf("notices = %q", got)
	}
}

// legacyContainer seeds a container created before the gateway binding.
func legacyContainer(fd *docker.Fake, st docker.ContainerState, withVolume bool) {
	fd.States[ContainerName] = st
	fd.PortBindings[ContainerName] = containerSpec(state.ModePersistent, Image, "").PortMappings
	if withVolume {
		fd.Mounts[ContainerName] = []docker.VolumeMount{{VolumeName: DataVolume, ContainerPath: "/data"}}
	}
	fd.Networks[ContainerName] = []string{"bridge", "caddy_shack"}
}

func assertNetworksKept(t *testing.T, fd *docker.Fake) {
	t.Helper()
	if got := strings.Join(fd.Networks[ContainerName], ","); got != "bridge,caddy_shack" {
		t.Errorf("networks after recreate = %q, want bridge,caddy_shack", got)
	}
}

func TestInstallRecreatesLegacyPersistentContainer(t *testing.T) {
	for _, st := range []docker.ContainerState{docker.StateRunning, docker.StateStopped} {
		t.Run(st.String(), func(t *testing.T) {
			a, fd, fc, notices := newNativeApp(t)
			seedInstall(t, a, state.ModePersistent)
			s, _ := state.Load(a.paths.StatePath)
			s.Install.Image = LatestImage // upgraded install: must not be downgraded
			_ = state.Save(a.paths.StatePath, s)
			legacyContainer(fd, st, true)

			if err := a.Install(context.Background(), InstallInput{}); err != nil {
				t.Fatalf("Install: %v", err)
			}
			if !containsCall(fd.Calls, "Remove:"+ContainerName) || !containsCall(fd.Calls, "CreateAndStart:"+ContainerName) {
				t.Fatalf("expected recreate, calls = %v", fd.Calls)
			}
			if containsCall(fd.Calls, "Start:"+ContainerName) {
				t.Errorf("recreated container should not also be started: %v", fd.Calls)
			}
			if containsCall(fd.Calls, "RemoveVolume:"+DataVolume) {
				t.Errorf("data volume must be preserved: %v", fd.Calls)
			}
			if fd.LastSpec.Image != LatestImage {
				t.Errorf("recreated with %q, want recorded image %q", fd.LastSpec.Image, LatestImage)
			}
			if len(fd.LastSpec.Volumes) != 1 || fd.LastSpec.Volumes[0].VolumeName != DataVolume {
				t.Errorf("recreated without data volume: %+v", fd.LastSpec.Volumes)
			}
			if smtpGatewayMissing(a.hostNetwork(), fd.PortBindings[ContainerName]) {
				t.Errorf("recreated container still lacks gateway binding")
			}
			if len(fc.registers) != 1 {
				t.Errorf("should re-register with caddy_shack (reattaches caddy_shack network), got %v", fc.registers)
			}
			assertNetworksKept(t, fd)
			if !strings.Contains(notices.String(), "recreating") {
				t.Errorf("notices = %q", notices.String())
			}
		})
	}
}

func TestInstallDoesNotRecreateWhenMailWouldBeLost(t *testing.T) {
	cases := []struct {
		name     string
		mode     state.Mode
		volume   bool
		ephem    bool
		wantHint string
	}{
		{"ephemeral", state.ModeEphemeral, false, true, "docker rm -f mailpitter && mailpitter install --ephemeral"},
		{"persistent without volume", state.ModePersistent, false, false, "docker rm -f mailpitter && mailpitter install\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, fd, _, notices := newNativeApp(t)
			seedInstall(t, a, tc.mode)
			legacyContainer(fd, docker.StateStopped, tc.volume)
			if err := a.Install(context.Background(), InstallInput{Ephemeral: tc.ephem}); err != nil {
				t.Fatalf("Install: %v", err)
			}
			if containsCall(fd.Calls, "Remove:"+ContainerName) {
				t.Errorf("must not recreate: %v", fd.Calls)
			}
			if !containsCall(fd.Calls, "Start:"+ContainerName) {
				t.Errorf("stopped container should still be started: %v", fd.Calls)
			}
			if !strings.Contains(notices.String(), tc.wantHint) {
				t.Errorf("notices missing %q:\n%s", tc.wantHint, notices.String())
			}
		})
	}
}

func TestInstallLeavesCurrentContainerAlone(t *testing.T) {
	a, fd, _, _ := newNativeApp(t)
	seedInstall(t, a, state.ModePersistent)
	fd.States[ContainerName] = docker.StateRunning
	fd.PortBindings[ContainerName] = containerSpec(state.ModePersistent, Image, testGateway).PortMappings
	if err := a.Install(context.Background(), InstallInput{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if containsCall(fd.Calls, "Remove:"+ContainerName) || containsCall(fd.Calls, "ContainerMounts:"+ContainerName) {
		t.Errorf("should not touch an up-to-date container: %v", fd.Calls)
	}
}

func TestInstallRecreateFailures(t *testing.T) {
	cases := map[string]func(fd *docker.Fake){
		"remove":       func(fd *docker.Fake) { fd.RemoveErr = errors.New("rm") },
		"volume":       func(fd *docker.Fake) { fd.EnsureVolumeErr = errors.New("vol") },
		"create/start": func(fd *docker.Fake) { fd.CreateAndStartErr = errors.New("run") },
		"reconnect":    func(fd *docker.Fake) { fd.ConnectNetworkErr = errors.New("connect") },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			a, fd, _, _ := newNativeApp(t)
			seedInstall(t, a, state.ModePersistent)
			legacyContainer(fd, docker.StateRunning, true)
			setup(fd)
			if err := a.Install(context.Background(), InstallInput{}); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestStartNotesMissingGatewayAndUFW(t *testing.T) {
	a, fd, _, notices := newNativeApp(t)
	stubHost(t, hostIPs, "ENABLED=yes\n", "*filter\n")
	legacyContainer(fd, docker.StateStopped, true)
	if err := a.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	out := notices.String()
	if !strings.Contains(out, "run: mailpitter install") {
		t.Errorf("missing recreate hint:\n%s", out)
	}
	if !strings.Contains(out, ufwRule(testGateway)) {
		t.Errorf("missing ufw rule:\n%s", out)
	}
	if containsCall(fd.Calls, "Remove:"+ContainerName) {
		t.Errorf("start must never recreate: %v", fd.Calls)
	}
}

func TestUpgradeRecreatesWithGateway(t *testing.T) {
	a, fd, _, _ := newNativeApp(t)
	a.mailpit = &fakeMailpitWithInfo{err: errors.New("down")}
	seedInstall(t, a, state.ModePersistent)
	legacyContainer(fd, docker.StateRunning, true)
	fd.ImageIDs[LatestImage] = "sha256:new"
	fd.ContainerImageIDs[ContainerName] = "sha256:old"
	if err := a.Upgrade(context.Background()); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if smtpGatewayMissing(a.hostNetwork(), fd.LastSpec.PortMappings) {
		t.Errorf("upgrade should publish the gateway binding: %+v", fd.LastSpec.PortMappings)
	}
	assertNetworksKept(t, fd)
}

func TestUpgradeUpToDateNotesMissingGateway(t *testing.T) {
	a, fd, _, notices := newNativeApp(t)
	a.mailpit = &fakeMailpitWithInfo{err: errors.New("down")}
	seedInstall(t, a, state.ModePersistent)
	legacyContainer(fd, docker.StateRunning, true)
	fd.ImageIDs[LatestImage] = "sha256:same"
	fd.ContainerImageIDs[ContainerName] = "sha256:same"
	if err := a.Upgrade(context.Background()); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if !strings.Contains(notices.String(), "run: mailpitter install") {
		t.Errorf("notices = %q", notices.String())
	}
}

func containsCall(calls []string, want string) bool {
	for _, c := range calls {
		if c == want {
			return true
		}
	}
	return false
}
