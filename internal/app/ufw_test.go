package app

import (
	"bufio"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
)

// userRules mirrors /etc/ufw/user.rules on the reference Omarchy host,
// including the conventional allow-docker-dns rule.
const userRulesDNSOnly = `*filter
### RULES ###

### tuple ### allow tcp 53317 0.0.0.0/0 any 0.0.0.0/0 in
-A ufw-user-input -p tcp --dport 53317 -j ACCEPT

### tuple ### allow udp 53 172.17.0.1 any 172.16.0.0/12 in comment=616c6c6f772d646f636b65722d646e73
-A ufw-user-input -p udp -d 172.17.0.1 --dport 53 -s 172.16.0.0/12 -j ACCEPT

### END RULES ###
`

const mailpitRule = `### tuple ### allow tcp 1025 172.17.0.1 any 172.16.0.0/12 in comment=616c6c6f772d646f636b65722d6d61696c706974
-A ufw-user-input -p tcp -d 172.17.0.1 --dport 1025 -s 172.16.0.0/12 -j ACCEPT
`

func TestUFWRule(t *testing.T) {
	want := "sudo ufw allow from 172.16.0.0/12 to 172.17.0.1 port 1025 proto tcp comment 'allow-docker-mailpit'"
	if got := ufwRule("172.17.0.1"); got != want {
		t.Errorf("ufwRule = %q", got)
	}
}

func TestFindSMTPAllow(t *testing.T) {
	cases := []struct {
		name, rules, want string
	}{
		{"dns rule only", userRulesDNSOnly, ""},
		{"mailpit rule", userRulesDNSOnly + mailpitRule, "172.16.0.0/12"},
		{"port open to all", "### tuple ### allow tcp 1025 0.0.0.0/0 any 0.0.0.0/0 in\n", "0.0.0.0/0"},
		{"any proto, port range", "### tuple ### allow any 1000:2000 172.17.0.0/16 any 10.0.0.0/8 in\n", "10.0.0.0/8"},
		{"port list", "### tuple ### allow tcp 25,1025 172.17.0.1 any 172.16.0.0/12 in\n", "172.16.0.0/12"},
		{"other gateway", "### tuple ### allow tcp 1025 172.18.0.1 any 172.16.0.0/12 in\n", ""},
		{"deny", "### tuple ### deny tcp 1025 172.17.0.1 any 172.16.0.0/12 in\n", ""},
		{"udp", "### tuple ### allow udp 1025 172.17.0.1 any 172.16.0.0/12 in\n", ""},
		{"outbound", "### tuple ### allow tcp 1025 172.17.0.1 any 172.16.0.0/12 out\n", ""},
		{"route rule", "### tuple ### route:allow tcp 1025 172.17.0.1 any 172.16.0.0/12 in\n", ""},
		{"truncated", "### tuple ### allow tcp 1025\n", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := findSMTPAllow(bufio.NewScanner(strings.NewReader(tc.rules)), testGateway)
			if err != nil || got != tc.want {
				t.Errorf("findSMTPAllow = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestPortListHasAndAddrCovers(t *testing.T) {
	for spec, want := range map[string]bool{"any": true, "1025": true, "25": false, "1:x": false, "x,1025": true, "2000:1000": false} {
		if portListHas(spec, 1025) != want {
			t.Errorf("portListHas(%q) != %v", spec, want)
		}
	}
	gw := net.ParseIP(testGateway)
	for spec, want := range map[string]bool{"172.17.0.1": true, "172.16.0.0/12": true, "10.0.0.0/8": false, "garbage": false} {
		if addrCovers(spec, gw) != want {
			t.Errorf("addrCovers(%q) != %v", spec, want)
		}
	}
	if addrCovers("0.0.0.0/0", nil) {
		t.Error("nil ip should never be covered")
	}
}

func TestReadUFWStatus(t *testing.T) {
	cases := []struct {
		name, conf, rules string
		want              UFWStatus
		wantErr           bool
	}{
		{"not installed", "", "", UFWStatus{}, false},
		{"disabled", "ENABLED=no\n", userRulesDNSOnly, UFWStatus{Installed: true}, false},
		{"active, no rule", "# comment\nENABLED=yes\nLOGLEVEL=low\n", userRulesDNSOnly, UFWStatus{Installed: true, Active: true}, false},
		{"active, rule", "ENABLED=yes\n", userRulesDNSOnly + mailpitRule, UFWStatus{Installed: true, Active: true, RuleSource: "172.16.0.0/12"}, false},
		{"rules unreadable", "ENABLED=yes\n", "", UFWStatus{Installed: true, Active: true}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubHost(t, nil, tc.conf, tc.rules)
			got, err := readUFWStatus(testGateway)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Errorf("readUFWStatus = %+v, %v", got, err)
			}
		})
	}
}

func TestNoteUFW(t *testing.T) {
	native := HostNetwork{Kind: EngineNativeLinux, Gateway: testGateway}
	cases := []struct {
		name, conf, rules string
		hn                HostNetwork
		want              string
	}{
		{"active without rule", "ENABLED=yes\n", userRulesDNSOnly, native, "ufw is active"},
		{"active with rule", "ENABLED=yes\n", userRulesDNSOnly + mailpitRule, native, ""},
		{"inactive", "ENABLED=no\n", userRulesDNSOnly, native, ""},
		{"unreadable", "ENABLED=yes\n", "", native, "could not read ufw config"},
		{"desktop", "ENABLED=yes\n", userRulesDNSOnly, HostNetwork{Kind: EngineLoopbackForwarded}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, _, _, notices := newTestAppWithNotices(t)
			stubHost(t, nil, tc.conf, tc.rules)
			a.noteUFW(tc.hn)
			got := notices.String()
			if tc.want == "" && got != "" {
				t.Errorf("expected no notice, got %q", got)
			}
			if tc.want != "" && (!strings.Contains(got, tc.want) || !strings.Contains(got, ufwRule(testGateway))) {
				t.Errorf("notice = %q", got)
			}
		})
	}
}

func stubDial(t *testing.T, err error) {
	t.Helper()
	orig := dialTCP
	dialTCP = func(string) error { return err }
	t.Cleanup(func() { dialTCP = orig })
}

func TestCheckSMTPGatewayBinding(t *testing.T) {
	cases := []struct {
		name  string
		setup func(fd *docker.Fake)
		want  Severity
	}{
		{"desktop", func(fd *docker.Fake) { fd.Engine = desktopEngine }, SeveritySkip},
		{"unknown", func(fd *docker.Fake) { fd.Engine = docker.EngineInfo{} }, SeveritySkip},
		{"absent", func(fd *docker.Fake) {}, SeveritySkip},
		{"legacy", func(fd *docker.Fake) { legacyContainer(fd, docker.StateRunning, true) }, SeverityFail},
		{"current", func(fd *docker.Fake) {
			fd.States[ContainerName] = docker.StateRunning
			fd.PortBindings[ContainerName] = containerSpec("persistent", Image, testGateway).PortMappings
		}, SeverityPass},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, fd, _, _ := newNativeApp(t)
			tc.setup(fd)
			r := checkSMTPGatewayBinding(a)
			if r.Status != tc.want {
				t.Errorf("status = %v (%s), want %v", r.Status, r.Summary, tc.want)
			}
			if r.Status == SeverityFail && (r.Fix == nil || r.Fix.Command != "mailpitter install") {
				t.Errorf("fail should offer the literal install fix, got %+v", r.Fix)
			}
		})
	}
}

func TestCheckSMTPGatewayReachable(t *testing.T) {
	cases := []struct {
		name, conf, rules string
		dialErr           error
		desktop           bool
		want              Severity
		wantDetail        string
	}{
		{"desktop", "", "", nil, true, SeveritySkip, ""},
		{"not listening", "", "", errors.New("refused"), false, SeverityFail, ""},
		{"ufw absent", "", "", nil, false, SeverityPass, ""},
		{"ufw active, no rule", "ENABLED=yes\n", userRulesDNSOnly, nil, false, SeverityWarn, ufwRule(testGateway)},
		{"ufw active, rule", "ENABLED=yes\n", userRulesDNSOnly + mailpitRule, nil, false, SeverityPass, ""},
		{"ufw unreadable", "ENABLED=yes\n", "", nil, false, SeverityWarn, ufwRule(testGateway)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, fd, _, _ := newNativeApp(t)
			stubHost(t, hostIPs, tc.conf, tc.rules)
			stubDial(t, tc.dialErr)
			if tc.desktop {
				fd.Engine = desktopEngine
			}
			r := checkSMTPGatewayReachable(a)
			if r.Status != tc.want {
				t.Errorf("status = %v (%s), want %v", r.Status, r.Summary, tc.want)
			}
			if tc.wantDetail != "" && !strings.Contains(strings.Join(r.Detail, "\n"), tc.wantDetail) {
				t.Errorf("detail = %v", r.Detail)
			}
			if r.Fix != nil {
				t.Errorf("ufw rule needs sudo; must not be an auto-fix: %+v", r.Fix)
			}
		})
	}
}
