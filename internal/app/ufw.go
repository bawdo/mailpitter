package app

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DockerSourcePool is the source range the suggested ufw rule admits.
// It covers Docker's default bridge and user-defined network pools in
// 172.16.0.0/12, matching the conventional allow-docker-dns rule.
const DockerSourcePool = "172.16.0.0/12"

// ufwDir is a seam over ufw's config directory. Both files read from it
// (ufw.conf, user.rules) are world-readable, so no sudo is needed.
var ufwDir = "/etc/ufw"

// UFWStatus is what mailpitter can learn about ufw without root.
type UFWStatus struct {
	Installed bool
	Active    bool
	// RuleSource is the source of an existing allow rule for tcp/1025 to
	// the gateway; empty when none was found.
	RuleSource string
}

// ufwRule is the exact command the user should run to admit container
// traffic to the gateway-published SMTP port.
func ufwRule(gateway string) string {
	return fmt.Sprintf("sudo ufw allow from %s to %s port %d proto tcp comment 'allow-docker-mailpit'",
		DockerSourcePool, gateway, SMTPPort)
}

// readUFWStatus reports whether ufw is enabled (ufw.conf ENABLED=yes) and
// whether user.rules already allows tcp/1025 to gateway.
func readUFWStatus(gateway string) (UFWStatus, error) {
	conf, err := os.ReadFile(filepath.Join(ufwDir, "ufw.conf"))
	if errors.Is(err, fs.ErrNotExist) {
		return UFWStatus{}, nil
	}
	if err != nil {
		return UFWStatus{}, err
	}
	st := UFWStatus{Installed: true}
	for _, line := range strings.Split(string(conf), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && k == "ENABLED" {
			st.Active = strings.EqualFold(strings.Trim(v, `"'`), "yes")
		}
	}
	f, err := os.Open(filepath.Join(ufwDir, "user.rules"))
	if err != nil {
		return st, err
	}
	defer func() { _ = f.Close() }()
	src, err := findSMTPAllow(bufio.NewScanner(f), gateway)
	st.RuleSource = src
	return st, err
}

// findSMTPAllow scans ufw user.rules for a "### tuple ###" allow rule
// admitting tcp/1025 inbound to gateway and returns its source. Tuple
// fields: action proto dport dst sport src direction [comment=...].
func findSMTPAllow(sc *bufio.Scanner, gateway string) (string, error) {
	gw := net.ParseIP(gateway)
	for sc.Scan() {
		rest, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "### tuple ###")
		if !ok {
			continue
		}
		f := strings.Fields(rest)
		if len(f) < 7 || f[0] != "allow" || f[6] != "in" {
			continue
		}
		if f[1] != "tcp" && f[1] != "any" {
			continue
		}
		if !portListHas(f[2], SMTPPort) || !addrCovers(f[3], gw) {
			continue
		}
		return f[5], nil
	}
	return "", sc.Err()
}

// portListHas reports whether a ufw port spec ("1025", "25,1025",
// "1000:2000", "any") includes port.
func portListHas(spec string, port int) bool {
	if spec == "any" {
		return true
	}
	for _, part := range strings.Split(spec, ",") {
		lo, hi, isRange := strings.Cut(part, ":")
		l, err := strconv.Atoi(lo)
		if err != nil {
			continue
		}
		h := l
		if isRange {
			if h, err = strconv.Atoi(hi); err != nil {
				continue
			}
		}
		if port >= l && port <= h {
			return true
		}
	}
	return false
}

// addrCovers reports whether a ufw address spec (IP or CIDR) includes ip.
func addrCovers(spec string, ip net.IP) bool {
	if ip == nil {
		return false
	}
	if _, n, err := net.ParseCIDR(spec); err == nil {
		return n.Contains(ip)
	}
	return net.ParseIP(spec).Equal(ip)
}

// noteUFW prints the ufw rule to add when ufw is active and no existing
// rule admits container traffic to the gateway-published SMTP port.
// It never runs sudo or edits the firewall.
func (a *App) noteUFW(hn HostNetwork) {
	if hn.Kind != EngineNativeLinux {
		return
	}
	st, err := readUFWStatus(hn.Gateway)
	switch {
	case err != nil:
		_, _ = fmt.Fprintf(a.notices,
			"could not read ufw config (%v); if ufw is active, allow containers to reach SMTP with:\n  %s\n",
			err, ufwRule(hn.Gateway))
	case st.Active && st.RuleSource == "":
		_, _ = fmt.Fprintf(a.notices,
			"ufw is active and will drop container traffic to %s:%d; allow it with:\n  %s\n",
			hn.Gateway, SMTPPort, ufwRule(hn.Gateway))
	}
}
