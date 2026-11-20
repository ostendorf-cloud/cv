package cv

import (
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// forbiddenContent returns one reason per kind of content that must never
// appear in a public tool entry, or nil. The repository is public and the
// homelab is a real network, so an entry never carries:
//
//   - a URL, with or without credentials, or a "mailto:" link: a tool entry
//     has no link field, and a URL names a host
//   - an e-mail address: an "@" after a character and before a dotted host
//     with a letter in it, or before "localhost"
//   - an IPv4 address in dotted decimal, dotted hexadecimal ("0x7f.0.0.1"),
//     integer (8 to 10 digits up to 4294967295) or hexadecimal integer form,
//     and an IPv6 address, also with a zone id
//   - a MAC address in the forms 00:11:22:33:44:55, 00-11-22-33-44-55 and
//     0011.2233.4455
//   - a UNC path such as \\nas01\share
//   - a port: ":8080" as a word of its own, "name:NN" as described below,
//     "port 22", "ports: 22", "22/tcp", "TCP/22", "udp 5004", "listens on 8443"
//   - a hostname, see hostnameIn
//
// What passes: product names (Home Assistant, Node.js, ASP.NET, Vue.js 3,
// C++, .NET, k8s, Wi-Fi 6E, 10 GbE, 192 GB RAM), versions (v1.2, Talos 1.9.2),
// times and ratios (12:30), C++ scopes (std::vector), abbreviations (z.B.,
// u.v.m., i.d.R., Ph.D.), file names with a known extension (vite.config.ts,
// cv.de.yaml) and image references with a tag that holds a letter, a dot or a
// hyphen (node:22-alpine, ubuntu:24.04, postgres:16-alpine).
//
// host:port is decided on a word of exactly one colon whose part after the
// colon is 2 to 5 digits. It is refused when the part before is empty, or has
// a letter and ends in a digit (nas01:22), or contains a dot (nas.x:22), or the
// number has 4 or more digits (Linux:2024, node-red:1880). It passes when the
// part before is only digits (12:30) and otherwise (postgres:16, ssh:22). A
// plain word with a 2 or 3 digit tag, such as postgres:16, is the same shape as
// a host with a port and is accepted. Write the tag with a letter, a dot or a
// hyphen, or none.
//
// Versions: this rule refuses nothing about a version, with two exceptions that
// follow from the shapes above: four numbers separated by dots (1.2.3.4) read
// as an IPv4 address, and a word with a 4 digit tag (Linux:2024) reads as a
// port. The rule the issue states forbids the exact version of anything
// reachable from the internet, and a validator cannot know what is reachable.
// Keeping such a version out is the owner's duty.
//
// Words are cut at every character that is not a letter, a digit or one of
// ". : - _", so backticks, emphasis, quotes, "=", "@", "|" and the like around
// a name do not hide it.
//
// IPv4 short forms follow inet_aton: two to four numeric parts, each decimal,
// leading zero octal or 0x hex. Four parts that parse are refused (0177.0.0.1).
// Two or three parts are version shaped (1.2, 24.04, 3.14.15) and pass, except
// when the first part is 127 (127.1) or a part is 0x hex.
//
// An "@" followed by a single label that holds a letter and a digit (root@nas01,
// git@nas01:org/repo) is refused as a host. "node@22" and "pnpm@latest" pass.
//
// The check is a heuristic. It cannot catch: a bare single label (nas01,
// localhost, a short host name without a dot), "user@host" with a letters only
// host, a name with a dot and a non-listed ending of two labels (nas.example,
// router.fritz), a name written with a spaced or defanged dot ("nas dot
// example", "nas[.]local"), an address with other separators ("192 168 1 1",
// "192-168-1-1"), a MAC address without separators, a short IPv4 form that is
// version shaped (10.1), a port or address spelled in words or split by prose,
// a plain word with a 2 or 3 digit tag such as postgres:16 or ssh:22, a port
// in a form not listed, a number with fewer than 2 digits as a port, a two
// label name ending in ".NET" written in capitals, and reachability or
// exposure of anything. It also gives false alarms: an 8 to 10 digit number
// (20261120), a four part version (1.2.3.4), a name with a 4 digit tag, a name
// ending in a digit with a number tag (Python3:10), a name written like a domain
// (Socket.io, asp.net), a file such as archive.tar.gz, a dotted package or
// namespace name (com.example.app, Microsoft.AspNetCore.App) and a version
// pinned with a letter and digit (pnpm@9x). It is a backstop for a slip, not a
// review.
func forbiddenContent(text string) []string {
	var reasons []string
	add := func(reason string) { reasons = append(reasons, reason) }

	if m := urlPattern.FindString(text); m != "" {
		if hasCredentials(m) {
			add("a URL with credentials is not allowed")
		} else {
			add("a URL is not allowed")
		}
	}
	if hasEmail(text) {
		add("an e-mail address is not allowed")
	}
	if containsIPv4(text) {
		add("an IPv4 address is not allowed")
	}
	if containsIPv6(text) {
		add("an IPv6 address is not allowed")
	}
	if macPattern.MatchString(text) {
		add("a MAC address is not allowed")
	}
	if uncPattern.MatchString(text) {
		add("a UNC path is not allowed")
	}
	if hasPort(text) {
		add("a port is not allowed")
	}
	if host := hostnameIn(text); host != "" {
		add("a hostname is not allowed: " + strconv.Quote(host))
	}
	return reasons
}

var (
	// urlPattern matches a scheme followed by "://" and the rest of the word,
	// or the "mailto:" scheme.
	urlPattern = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://\S*|\bmailto:\S*`)

	// ipv4Pattern matches four dot separated decimal numbers that are not part
	// of a longer run of numbers and dots.
	ipv4Pattern = regexp.MustCompile(`(?:^|[^0-9.])([0-9]{1,3}(?:\.[0-9]{1,3}){3})(?:[^0-9.]|\.(?:[^0-9]|$)|$)`)

	// hexQuadPattern matches four dot separated parts, each decimal or 0x hex.
	hexQuadPattern = regexp.MustCompile(`^(?:0[xX][0-9A-Fa-f]{1,8}|[0-9]{1,3})(?:\.(?:0[xX][0-9A-Fa-f]{1,8}|[0-9]{1,3})){3}$`)

	// hexIntPattern matches a 32 bit hexadecimal number.
	hexIntPattern = regexp.MustCompile(`^0[xX][0-9A-Fa-f]{8}$`)

	// emailPattern matches "x@host" with a dotted host or "localhost".
	emailPattern = regexp.MustCompile(`[^\s@]@((?:[A-Za-z0-9-]+\.)+[A-Za-z0-9-]+|(?i:localhost))`)

	// singleHostPattern matches "x@label": a host without a dot after an "@".
	singleHostPattern = regexp.MustCompile(`[^\s@]@([\p{L}\p{N}_-]+)`)

	// macPattern matches the three common MAC address notations. A group of
	// the colon and hyphen forms may have one digit.
	macPattern = regexp.MustCompile(`(?:^|[^0-9A-Za-z])(?:[0-9A-Fa-f]{1,2}(?:[:-][0-9A-Fa-f]{1,2}){5}|[0-9A-Fa-f]{4}(?:\.[0-9A-Fa-f]{4}){2})(?:[^0-9A-Za-z]|$)`)

	// uncPattern matches a UNC path start, two backslashes and a name.
	uncPattern = regexp.MustCompile(`\\\\[^\s\\]`)

	// portWordPattern matches "port 22", "ports: 22", "port=22", "22/tcp",
	// "TCP/22", "udp 5004" and "listens on 8443".
	portWordPattern = regexp.MustCompile(`(?i)\bports?\b[\s:=#]*[0-9]{2,5}\b|\b[0-9]{1,5}/(?:tcp|udp)\b|\b(?:tcp|udp)[/ :][0-9]{2,5}\b|\blisten(?:s|ing)?(?:\s+on)?\s+:?[0-9]{2,5}\b`)

	// hostLabels matches a word made only of host labels joined by dots. A
	// label holds letters and digits of any script, so a non ASCII name matches.
	hostLabels = regexp.MustCompile(`^[\p{L}\p{N}_](?:[\p{L}\p{N}_-]*[\p{L}\p{N}_])?(?:\.[\p{L}\p{N}_](?:[\p{L}\p{N}_-]*[\p{L}\p{N}_])?)+$`)

	// onlyLetters matches a label that is a possible top level domain.
	onlyLetters = regexp.MustCompile(`^[A-Za-z]{2,}$`)
)

// internalSuffixes are the last labels that mark a private name.
var internalSuffixes = map[string]bool{
	"local": true, "lan": true, "internal": true, "intranet": true, "corp": true,
	"home": true, "private": true, "localdomain": true, "arpa": true,
	"box": true, "hole": true,
}

// knownTLDs are the endings that refuse even a two label name (example.com).
// ".NET" in capitals is exempt, see hostnameIn.
var knownTLDs = map[string]bool{
	"com": true, "org": true, "net": true, "de": true, "dev": true, "io": true,
	"cloud": true, "tech": true, "eu": true, "ch": true, "at": true, "uk": true,
	"app": true,
}

// fileExtensions are the last labels of a file name, which passes even with
// three labels (vite.config.ts). An internal suffix still wins: config.local.
var fileExtensions = map[string]bool{
	"ts": true, "js": true, "mjs": true, "json": true, "yaml": true, "yml": true,
	"toml": true, "md": true, "txt": true, "conf": true, "cfg": true, "sh": true,
	"go": true, "py": true, "env": true, "lock": true,
}

// words splits text into the words the word based checks look at. A word is a
// run of letters, digits and ". : - _": every other character, such as a
// backtick, an asterisk, a quote, "=", "@", "|", a bracket or an ellipsis, ends
// a word, so formatting around a name does not hide it.
func words(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune(".:-_", r)
	})
}

// trimEdges removes the characters a word may carry at its ends but that are
// never part of a name or an address.
func trimEdges(w string) string { return strings.Trim(w, ".-_") }

// partValue parses one part of a dotted IPv4 form the way inet_aton does:
// 0x hex, leading zero octal or decimal. hex reports the 0x form.
func partValue(s string) (value uint64, hex, ok bool) {
	var err error
	switch {
	case len(s) > 2 && (s[:2] == "0x" || s[:2] == "0X"):
		value, err = strconv.ParseUint(s[2:], 16, 32)
		hex = true
	case len(s) > 1 && s[0] == '0':
		value, err = strconv.ParseUint(s[1:], 8, 32)
	default:
		value, err = strconv.ParseUint(s, 10, 32)
	}
	return value, hex, err == nil
}

// isIPv4Form reports an inet_aton form that the check refuses: four parts that
// parse, or two or three parts that parse with a first part of 127 or a 0x
// part. Two or three plain parts are version shaped and pass.
func isIPv4Form(w string) bool {
	parts := strings.Split(w, ".")
	n := len(parts)
	if n < 2 || n > 4 {
		return false
	}
	anyHex := false
	var first uint64
	for i, p := range parts {
		v, hex, ok := partValue(p)
		if !ok {
			return false
		}
		limit := uint64(255)
		if i == n-1 {
			limit = 1<<(8*uint(5-n)) - 1
		}
		if v > limit {
			return false
		}
		anyHex = anyHex || hex
		if i == 0 {
			first = v
		}
	}
	return n == 4 || first == 127 || anyHex
}

func hasCredentials(url string) bool {
	_, rest, _ := strings.Cut(url, "://")
	authority, _, _ := strings.Cut(rest, "/")
	return strings.Contains(authority, "@")
}

// hasEmail reports an "@" before a dotted host that has a letter in it, so
// "node@22" and "go@1.22" pass and "admin@nas.local" does not.
func hasEmail(text string) bool {
	for _, m := range emailPattern.FindAllStringSubmatch(text, -1) {
		if strings.IndexFunc(m[1], unicode.IsLetter) >= 0 {
			return true
		}
	}
	for _, m := range singleHostPattern.FindAllStringSubmatch(text, -1) {
		if strings.IndexFunc(m[1], unicode.IsLetter) >= 0 && strings.IndexFunc(m[1], unicode.IsDigit) >= 0 {
			return true
		}
	}
	return false
}

func containsIPv4(text string) bool {
	for _, m := range ipv4Pattern.FindAllStringSubmatch(text, -1) {
		valid := true
		for _, octet := range strings.Split(m[1], ".") {
			if n, err := strconv.Atoi(octet); err != nil || n > 255 {
				valid = false
			}
		}
		if valid {
			return true
		}
	}
	for _, w := range words(text) {
		w = strings.Trim(trimEdges(w), ":")
		switch {
		case isIPv4Form(w):
			return true
		case hexIntPattern.MatchString(w):
			return true
		case hexQuadPattern.MatchString(w) && strings.Contains(strings.ToLower(w), "0x"):
			return true
		case len(w) >= 8 && len(w) <= 10 && allDigits(w):
			if n, err := strconv.ParseUint(w, 10, 64); err == nil && n <= 4294967295 {
				return true
			}
		}
	}
	return false
}

func containsIPv6(text string) bool {
	for _, w := range words(text) {
		w = trimEdges(w)
		if strings.Count(w, ":") < 2 || strings.Trim(w, ":") == "" {
			continue
		}
		if addr, err := netip.ParseAddr(w); err == nil && addr.Is6() {
			return true
		}
	}
	return false
}

func allDigits(s string) bool {
	return s != "" && strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) < 0
}

// hasPort reports a port in any of the notations forbiddenContent lists.
func hasPort(text string) bool {
	if portWordPattern.MatchString(text) {
		return true
	}
	for _, w := range words(text) {
		if hostPort(trimEdges(w)) {
			return true
		}
	}
	return false
}

// hostPort decides one word of the form "before:digits", see forbiddenContent.
func hostPort(w string) bool {
	before, num, ok := strings.Cut(w, ":")
	if !ok || strings.Contains(num, ":") || len(num) < 2 || len(num) > 5 || !allDigits(num) {
		return false
	}
	switch {
	case before == "":
		return true
	case allDigits(before):
		return false
	case len(num) >= 4, strings.Contains(before, "."):
		return true
	}
	last := before[len(before)-1]
	hasLetter := strings.IndexFunc(before, unicode.IsLetter) >= 0
	return hasLetter && last >= '0' && last <= '9'
}

// hostnameIn returns the first word of text that is shaped like a host name,
// or "". A word is a host name when its labels are joined by dots and
//
//   - a label starts with "xn--" (punycode), or
//   - the last label is an internal suffix (.local, .lan, .internal, .intranet,
//     .corp, .home, .private, .localdomain, .arpa, .box, .hole): "config.local" and
//     ".env.local" are refused because "local" is an internal suffix. The
//     suffix and the file extension lists share no entry, so the order of the
//     two checks makes no difference, or
//   - the last label is a file extension (vite.config.ts, cv.de.yaml): passes, or
//   - the word has a non ASCII letter: refused, or
//   - the last label is a known TLD (com, org, net, de, dev, io, cloud, tech,
//     eu, ch, at, uk, app), also with two labels: "example.com" is refused,
//     except a last label written exactly ".NET", so "ASP.NET" passes, or
//   - there are three or more labels and the last has 2 or more letters.
//
// Node.js, Vue.js, Ph.D., z.B. and i.d.R. pass. Socket.io and a lowercase
// asp.net are refused, as is a product written like a domain.
func hostnameIn(text string) string {
	for _, w := range words(text) {
		w = strings.Trim(w, ".:-_")
		if !hostLabels.MatchString(w) {
			continue
		}
		labels := strings.Split(strings.ToLower(w), ".")
		last := labels[len(labels)-1]
		punycode := false
		for _, l := range labels {
			punycode = punycode || strings.HasPrefix(l, "xn--")
		}
		switch {
		case punycode, internalSuffixes[last]:
			return w
		case fileExtensions[last]:
			continue
		case strings.IndexFunc(w, func(r rune) bool { return r > unicode.MaxASCII }) >= 0:
			return w
		case knownTLDs[last] && !strings.HasSuffix(w, ".NET"):
			return w
		case len(labels) >= 3 && onlyLetters.MatchString(last):
			return w
		}
	}
	return ""
}
