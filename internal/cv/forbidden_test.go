package cv

import (
	"strings"
	"testing"
)

// TestForbiddenContentRefuses lists what the rule must catch, one sample per
// shape, with a fragment of the reason it must give.
func TestForbiddenContentRefuses(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		text string
		want string
	}{
		"ipv4 address":                {"Runs on 192.168.1.10", "IPv4"},
		"ipv4 in a sentence end":      {"Reachable at 10.0.0.1.", "IPv4"},
		"ipv4 with a prefix length":   {"Subnet 10.0.0.0/24", "IPv4"},
		"ipv4 with a port":            {"Dashboard 10.0.0.5:3000", "IPv4"},
		"four number version is ipv4": {"Firmware v1.2.3.4", "IPv4"},
		"ipv6 loopback":               {"Listens on ::1", "IPv6"},
		"ipv6 full":                   {"Host 2001:db8:85a3:0:0:8a2e:370:7334", "IPv6"},
		"ipv6 compressed":             {"Host 2001:db8::1", "IPv6"},
		"ipv6 link local":             {"Host fe80::1", "IPv6"},
		"ipv6 in brackets with port":  {"Host [2001:db8::1]:8443", "IPv6"},
		"ipv6 at a sentence end":      {"Host fe80::1.", "IPv6"},
		"port after a host word":      {"Served on proxy:8080", "port"},
		"port with a leading colon":   {"Listens on :8080", "port"},
		"port in words":               {"SSH on port 22", "port"},
		"port with a colon":           {"Ports: 443 and 80", "port"},
		"port with an equals sign":    {"port=9000", "port"},
		"port after a bracket":        {"[::1]:80", "port"},
		"internal suffix local":       {"Reachable as nas.local", "hostname"},
		"internal suffix lan":         {"Reachable as router.lan", "hostname"},
		"internal suffix internal":    {"Reachable as git.internal", "hostname"},
		"internal suffix home.arpa":   {"Reachable as printer.home.arpa", "hostname"},
		"internal suffix intranet":    {"wiki.intranet", "hostname"},
		"fully qualified name":        {"Runs at nas.example.com", "hostname"},
		"name with a trailing dot":    {"Runs at nas.example.com.", "hostname"},
		"name inside parentheses":     {"Backup (nas.example.com)", "hostname"},
		"url":                         {"See https://example.com/tools", "URL"},
		"url with credentials":        {"Clone https://user:secret@example.com/repo", "credentials"},
		"url with a user only":        {"ssh://git@example.com/repo", "credentials"},
		"ftp url":                     {"ftp://example.com", "URL"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := forbiddenContent(tc.text)
			if len(got) == 0 {
				t.Fatalf("forbiddenContent(%q) = nil, want a reason with %q", tc.text, tc.want)
			}
			if !strings.Contains(strings.Join(got, "; "), tc.want) {
				t.Errorf("forbiddenContent(%q) = %v, want a reason with %q", tc.text, got, tc.want)
			}
		})
	}
}

// TestForbiddenContentAccepts lists what must pass: product names, prose and
// versions the rule deliberately does not judge.
func TestForbiddenContentAccepts(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"empty":                         "",
		"plain product name":            "Home Assistant",
		"dotted product Node.js":        "Node.js",
		"dotted product ASP.NET":        "ASP.NET Core",
		"dotted product Vue.js":         "Vue.js 3 for small front ends",
		"dotted product Next.js":        "Next.js",
		"product with a plus":           "C++ and Rust",
		"cpp namespace":                 "Uses std::vector a lot",
		"cpp scope operator":            "Reads :: as scope",
		"short version":                 "v1.2",
		"three part version":            "Talos 1.9.2",
		"version in prose":              "Kubernetes 1.31 on three nodes",
		"five part version":             "build 1.2.3.4.5",
		"time of day":                   "Backups run at 12:30",
		"ratio":                         "A 1:100 scale model",
		"sentence with a colon":         "Note: this is a note",
		"sentence with a dot":           "Used daily. Works well.",
		"abbreviation e.g.":             "Tools, e.g. editors",
		"the word port in prose":        "A port of the tool to Go",
		"the word ports in prose":       "Ports and adapters",
		"single digit port wording":     "Thunderbolt port 4",
		"hardware model":                "Ubiquiti UniFi Dream Machine Pro",
		"numbers with a unit":           "64 GB RAM, 2 TB NVMe",
		"two label name":                "example.com",
		"filename":                      "Edits the config.yaml file",
		"abbreviation with dots":        "Written in K.I.S.S. style",
		"homelab platform product name": "Proxmox VE and Talos Linux",
		"numeric octet over 255":        "Model 999.999.999.999",
	}

	for name, text := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := forbiddenContent(text); got != nil {
				t.Errorf("forbiddenContent(%q) = %v, want nil", text, got)
			}
		})
	}
}

// TestForbiddenContentReportsEveryKind proves one text with several kinds of
// forbidden content gets one reason per kind.
func TestForbiddenContentReportsEveryKind(t *testing.T) {
	t.Parallel()

	got := forbiddenContent("nas.example.com at 10.0.0.1 on port 22")
	if len(got) != 3 {
		t.Fatalf("forbiddenContent() = %v, want three reasons", got)
	}
}
