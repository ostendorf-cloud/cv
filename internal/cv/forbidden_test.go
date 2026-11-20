package cv

import (
	"fmt"
	"strings"
	"testing"
	"time"
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

		"upper case url scheme":        {"HTTPS://Example.com", "URL"},
		"mailto link":                  {"mailto:admin@nas.local", "URL"},
		"email with a local host":      {"admin@nas.local", "e-mail"},
		"email with a public host":     {"simon@example.com", "e-mail"},
		"email at localhost":           {"root@localhost", "e-mail"},
		"mac with colons":              {"NIC 00:11:22:33:44:55", "MAC"},
		"mac with hyphens":             {"NIC 00-11-22-33-44-55", "MAC"},
		"mac in dotted form":           {"NIC 0011.2233.4455", "MAC"},
		"unc path":                     {`Share \\nas01\share`, "UNC"},
		"slash tcp":                    {"Opens 22/tcp", "port"},
		"tcp slash":                    {"Opens TCP/22", "port"},
		"udp with a space":             {"Stream on udp 5004", "port"},
		"listens on":                   {"listens on 8443", "port"},
		"digit host with a port":       {"nas01:22", "port"},
		"upper digit host with a port": {"NAS01:22", "port"},
		"dotted word with a port":      {"my.box:22", "port"},
		"four digit port":              {"node-red:1880", "port"},
		"four digit number after word": {"Linux:2024", "port"},
		"port after a path name":       {"example.com:8080/x", "port"},
		"scheme less path":             {"example.com/path", "hostname"},
		"two label com":                {"example.com", "hostname"},
		"two label io":                 {"Socket.io", "hostname"},
		"two label org upper":          {"EXAMPLE.ORG", "hostname"},
		"two label lowercase net":      {"asp.net", "hostname"},
		"fritz box":                    {"fritz.box", "hostname"},
		"pi hole":                      {"pi.hole", "hostname"},
		"corp suffix":                  {"mail.corp", "hostname"},
		"arpa suffix":                  {"1.0.0.127.in-addr.arpa", "hostname"},
		"capital internal suffix":      {"NAS.LOCAL", "hostname"},
		"env local file":               {".env.local", "hostname"},
		"config local file":            {"config.local", "hostname"},
		"punycode":                     {"xn--nas-abc.xn--def", "hostname"},
		"non ascii host":               {"bücher.example", "hostname"},
		"non ascii host with tld":      {"münchen.de", "hostname"},
		"octet 255":                    {"Mask 255.255.255.255", "IPv4"},
		"integer ipv4":                 {"Host 3232235777", "IPv4"},
		"hex integer ipv4":             {"Host 0x7f000001", "IPv4"},
		"hex dotted ipv4":              {"Host 0x7f.0.0.1", "IPv4"},
		"ipv6 zone id":                 {"Host fe80::1%eth0", "IPv6"},

		"two label arpa":             {"nas.arpa", "hostname"},
		"two label home arpa":        {"home.arpa", "hostname"},
		"underscore in a label":      {"my_nas.local", "hostname"},
		"leading underscore":         {"_nas.local", "hostname"},
		"short loopback":             {"Host 127.1", "IPv4"},
		"three part loopback":        {"Host 127.0.1", "IPv4"},
		"octal ipv4":                 {"Host 0177.0.0.1", "IPv4"},
		"hex short ipv4":             {"Host 0x7f.1", "IPv4"},
		"single label host at":       {"root@nas01", "e-mail"},
		"single label host with git": {"git@nas01:org/repo", "e-mail"},
		"one digit mac group":        {"NIC 0:11:22:33:44:55", "MAC"},

		"host glued with a hyphen":      {"Der nas.local-Rechner", "hostname"},
		"fqdn glued with a hyphen":      {"nas.example.com-Instanz", "hostname"},
		"lan glued with a hyphen":       {"router.lan-Zugang", "hostname"},
		"internal glued with a hyphen":  {"git.internal-Repo", "hostname"},
		"tld name glued with a hyphen":  {"example.com-Domain", "hostname"},
		"host glued with an underscore": {"nas.local_backup", "hostname"},
		"ipv6 glued with a hyphen":      {"fe80::1-Adresse", "IPv6"},
		"loopback glued with a hyphen":  {"::1-Loopback", "IPv6"},
		"ipv6 glued, long":              {"2001:db8::1-Netz", "IPv6"},
		"ipv6 glued with an underscore": {"fe80::1_x", "IPv6"},
		"port glued with a hyphen":      {":8080-Port", "port"},
		"host port glued with a hyphen": {"nas01:22-Port", "port"},
		"dotted host port glued":        {"nas.local:22-Weiterleitung", "port"},
		"ipv4 glued with a hyphen":      {"10.0.0.1-Netz", "IPv4"},
		"zero width space in a host":    {"nas\u200b.local", "hostname"},
		"soft hyphen in a host":         {"nas.lo\u00adcal", "hostname"},
		"fullwidth host":                {"ｎａｓ．ｌｏｃａｌ", "hostname"},
		"fullwidth dot":                 {"nas\uff0elocal", "hostname"},
		"ideographic dot":               {"nas。local", "hostname"},
		"halfwidth ideographic dot":     {"nas｡local", "hostname"},
		"one dot leader":                {"nas․local", "hostname"},
		"dot leader address":            {"10․0․0․1", "IPv4"},
		"fullwidth dot address":         {"192．168．1．1", "IPv4"},
		"small full stop":               {"nas\ufe52local", "hostname"},
		"zero width space in a scheme":  {"https\u200b://x", "URL"},
		"no break space in port":        {"Port\u00a022", "port"},
		"no break space before tcp":     {"22\u00a0/tcp", "port"},
		"no break space after udp":      {"udp\u00a05004", "port"},
		"no break space in listens":     {"listens\u00a0on\u00a08443", "port"},
		"ideographic space after port":  {"port\u300022", "port"},
		"two label dev":                 {"nas.dev", "hostname"},
		"two label cloud":               {"nas.cloud", "hostname"},
		"two label tech":                {"nas.tech", "hostname"},
		"two label eu":                  {"nas.eu", "hostname"},
		"two label ch":                  {"nas.ch", "hostname"},
		"two label at":                  {"nas.at", "hostname"},
		"two label uk":                  {"nas.uk", "hostname"},
		"two label app":                 {"nas.app", "hostname"},
		"two label de":                  {"nas.de", "hostname"},
		"two label org":                 {"nas.org", "hostname"},
		"two label net":                 {"nas.net", "hostname"},
		"two label io, ascii name":      {"nas.io", "hostname"},
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
		"two label name, odd ending":    "nas.example",
		"capital NET ending":            "ASP.NET and .NET",
		"cpp":                           "C++",
		"k8s":                           "k8s",
		"wifi":                          "Wi-Fi 6E",
		"ethernet":                      "10 GbE",
		"memory":                        "192 GB RAM",
		"german abbreviation zB":        "z.B.",
		"german abbreviation uvm":       "u.v.m.",
		"german abbreviation idR":       "i.d.R.",
		"academic title":                "Ph.D.",
		"image tag with letters":        "postgres:16-alpine",
		"image tag node":                "node:22-alpine",
		"image tag with a dot":          "ubuntu:24.04",
		"plain word with a short tag":   "postgres:16",
		"file name ts":                  "vite.config.ts",
		"file name json":                "tsconfig.app.json",
		"file name yaml":                "cv.de.yaml",
		"file name lock":                "pnpm-lock.yaml and Cargo.lock",
		"octet 256":                     "Model 10.0.0.256",
		"short number":                  "Intel 12900K",
		"npm scoped package":            "@angular/core",
		"pinned with an at sign":        "node@22 and go@1.22",
		"hex number in prose":           "Colour 0xFF",
		"mac like short hex":            "AA:BB:CC",
		"product glued with a hyphen":   "Node.js-Server and Vue.js-Projekte",
		"version glued with a hyphen":   "Python 3.12-Umgebung and Ubuntu 24.04-LTS",
		"cpp scope glued":               "std::vector-Typen",
		"cpp glued":                     "C++-Code",
		"dotnet glued":                  ".NET-Anwendung and ASP.NET-Server",
		"k8s glued":                     "k8s-Cluster",
		"ci cd glued":                   "CI/CD-Pipeline",
		"hyphenated names":              "Kolla-Ansible, Open-Source, Apache-2.0, GPL-3.0-or-later",
		"emoji with a joiner":           "Familie " + string([]rune{0x1F468, 0x200d, 0x1F469, 0x200d, 0x1F467}) + " z.B. so",
		"no break space in prose":       "Port und Adapter, 10 GbE",
		"two part version":              "Kubernetes 1.31 and v1.2",
		"version with a leading zero":   "Ubuntu 24.04",
		"three part version, small":     "Go 3.14.15",
		"date with dots":                "Release 2026.11.20",
		"pinned package":                "pnpm@latest and node@22",
		"emphasised product":            "**Home Assistant** and `Node.js` and *Vue.js*",
		"quoted product":                "„Node.js“ and ‚ASP.NET‘",
		"ellipsis after a product":      "Node.js… and Vue.js 3",
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

// TestForbiddenContentSeesThroughFormatting proves that punctuation, markup
// and quotes around a forbidden word do not hide it.
func TestForbiddenContentSeesThroughFormatting(t *testing.T) {
	t.Parallel()

	bare := []string{
		"nas.local", "example.com", "nas.example.com", "::1", "2001:db8::1", "fe80::1", ":8080", "nas01:22",
	}
	wrappers := []string{
		"`%s`", "*%s*", "**%s**", "_%s_", "=%s", "host=%s", "„%s“", "«%s»", "%s…", "%s|", "@%s",
		"~%s", "#%s", "$%s", "%%%s", "&%s", "+%s", "(%s)", "[%s]", "\"%s\"", "'%s'", "<%s>", "%s.",
		"%s!", "%s?", "%s;", "%s,", "%s-Zusatz", "%s_x", "Neu-%s", "x_%s",
	}
	for _, b := range bare {
		for _, w := range wrappers {
			text := fmt.Sprintf("Reach it at "+w+" daily", b)
			if forbiddenContent(text) == nil {
				t.Errorf("forbiddenContent(%q) = nil, want a reason", text)
			}
		}
	}
}

// TestForbiddenContentIsFastOnLongInput runs adversarial inputs of 5000
// and 20000 characters and logs both times. The patterns are RE2 and the word
// splitting is one pass, so four times the input takes about four times as
// long. The test fails on a ratio above 8, which a quadratic scan reaches, and
// on an absolute bound far above the measured time.
func TestForbiddenContentIsFastOnLongInput(t *testing.T) {
	t.Parallel()

	units := []string{
		"a", "a.", "1.", ":", "a:1", "0x", "a@", "a b ", "-", "_", "a-", "a_", "a.b-", "1-", "::-",
		"00:11:", `\`, "ü.", string(rune(0x200b)) + ".", string(rune(0xff0e)),
	}
	const (
		size       = 5000
		factor     = 4                     // the large input is factor times the small one
		noise      = 20 * time.Millisecond // below this the ratio says nothing
		ratioLimit = 8                     // linear is about 4, quadratic about 16
		bound      = 2 * time.Second       // absolute bound per input, far above the measured time
	)
	// measure takes the fastest of two runs, which removes scheduler noise.
	measure := func(in string) time.Duration {
		best := time.Duration(1<<63 - 1)
		for range 2 {
			start := time.Now()
			forbiddenContent(in)
			best = min(best, time.Since(start))
		}
		return best
	}
	for _, unit := range units {
		unit := unit
		small := measure(strings.Repeat(unit, size/len([]rune(unit))))
		large := measure(strings.Repeat(unit, factor*size/len([]rune(unit))))
		t.Logf("%q: %d characters %s, %d characters %s", unit, size, small, factor*size, large)
		if large > bound {
			t.Errorf("%q: %d characters took %s, want under %s", unit, factor*size, large, bound)
		}
		if small > noise && large > ratioLimit*small {
			t.Errorf("%q: %dx the input took %s to %s, want linear time", unit, factor, small, large)
		}
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
