package cv

import (
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

// forbiddenContent returns one reason per kind of content that must never
// appear in a public tool entry, or nil. The repository is public and the
// homelab is a real network, so an entry never carries:
//
//   - a URL, with or without credentials: a tool entry has no link field,
//     and a URL names a host
//   - an IPv4 or IPv6 address
//   - a port, as "host:8080", ":8080" or "port 22"
//   - a hostname: three or more dot separated labels that end in letters
//     (nas.example.com), or any name ending in an internal suffix
//     (.local, .lan, .internal, .home.arpa and the like)
//
// A product name passes: "Home Assistant", "Node.js", "ASP.NET", "Vue.js 3".
//
// Versions: this rule refuses nothing about a version, with one exception. A
// version that is four numbers separated by dots, such as "10.0.0.1" or
// "v1.2.3.4", is indistinguishable from an IPv4 address and is refused. A
// version such as "v1.2" or "1.2.3" passes. The rule the issue states forbids
// the exact version of anything reachable from the internet, and a validator
// cannot know what is reachable. Keeping such a version out is the owner's
// duty.
//
// The check is a heuristic. It cannot see a bare hostname such as "nas01" or
// a port written out in words. It is a backstop for a slip, not a review.
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
	if containsIPv4(text) {
		add("an IPv4 address is not allowed")
	}
	if containsIPv6(text) {
		add("an IPv6 address is not allowed")
	}
	if portPattern.MatchString(text) {
		add("a port is not allowed")
	}
	if host := firstHostname(text); host != "" {
		add("a hostname is not allowed: " + strconv.Quote(host))
	}
	return reasons
}

var (
	// urlPattern matches a scheme followed by "://" and the rest of the word.
	urlPattern = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://\S*`)

	// ipv4Pattern matches four dot separated numbers that are not part of a
	// longer run of numbers and dots.
	ipv4Pattern = regexp.MustCompile(`(?:^|[^0-9.])([0-9]{1,3}(?:\.[0-9]{1,3}){3})(?:[^0-9.]|\.(?:[^0-9]|$)|$)`)

	// portPattern matches "name:8080", "]:8080" and ":8080", and "port 22" in
	// the forms "port 22", "ports: 22" and "port=22". A digit before the
	// colon does not match, so a time such as "12:30" passes.
	portPattern = regexp.MustCompile(`(?i)(?:(?:^|[\s\]A-Za-z)]):[0-9]{2,5}\b|\bports?\b[\s:=#]*[0-9]{2,5}\b)`)

	// hostLabels matches a token made only of host labels joined by dots.
	hostLabels = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?)+$`)

	// onlyLetters matches a label that is a possible top level domain.
	onlyLetters = regexp.MustCompile(`^[A-Za-z]{2,}$`)
)

// internalSuffixes are the last labels that mark a private name.
var internalSuffixes = []string{
	"local", "lan", "internal", "intranet", "corp", "home", "private", "localdomain", "arpa",
}

func hasCredentials(url string) bool {
	_, rest, _ := strings.Cut(url, "://")
	authority, _, _ := strings.Cut(rest, "/")
	return strings.Contains(authority, "@")
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
	return false
}

func containsIPv6(text string) bool {
	words := strings.FieldsFunc(text, func(r rune) bool {
		return strings.ContainsRune(" \t\r\n,;()[]{}\"'<>/", r)
	})
	for _, w := range words {
		w = strings.TrimRight(w, ".!?")
		if strings.Count(w, ":") < 2 || strings.Trim(w, ":") == "" {
			continue
		}
		if addr, err := netip.ParseAddr(w); err == nil && addr.Is6() {
			return true
		}
	}
	return false
}

// firstHostname returns the first word of text that is shaped like a host
// name, or "".
func firstHostname(text string) string {
	words := strings.FieldsFunc(text, func(r rune) bool {
		return strings.ContainsRune(" \t\r\n,;()[]{}\"'<>/", r)
	})
	for _, w := range words {
		w = strings.Trim(w, ".:!?-")
		if !hostLabels.MatchString(w) {
			continue
		}
		labels := strings.Split(strings.ToLower(w), ".")
		last := labels[len(labels)-1]
		if hasInternalSuffix(labels) || (len(labels) >= 3 && onlyLetters.MatchString(last)) {
			return w
		}
	}
	return ""
}

func hasInternalSuffix(labels []string) bool {
	last := labels[len(labels)-1]
	for _, s := range internalSuffixes {
		if last == s {
			return true
		}
	}
	return false
}
