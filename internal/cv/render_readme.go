package cv

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// The markers delimit the generated block inside a profile README that also
// carries hand written text. They are HTML comments, so GitHub does not show
// them.
const (
	ReadmeBeginMarker = "<!-- cv:begin -->"
	ReadmeEndMarker   = "<!-- cv:end -->"
)

// The ways ReplaceReadmeBlock refuses a README.
var (
	// ErrReadmeMarkerMissing means the begin or the end marker is absent.
	ErrReadmeMarkerMissing = errors.New("readme marker missing")
	// ErrReadmeMarkerDuplicate means a marker occurs more than once.
	ErrReadmeMarkerDuplicate = errors.New("readme marker duplicated")
	// ErrReadmeMarkerOrder means the end marker comes before the begin marker.
	ErrReadmeMarkerOrder = errors.New("readme markers out of order")
	// ErrReadmeMarkerInContent means CV text carries a marker, which would
	// split the block on the next replacement.
	ErrReadmeMarkerInContent = errors.New("cv text contains a readme marker")
	// ErrReadmeUnsafeContent means CV text would break the README around the
	// block: an HTML comment opener, or a project repository that is not
	// owner/name.
	ErrReadmeUnsafeContent = errors.New("cv text unsafe in a readme")
)

// repoName is the shape of a GitHub repository, owner/name.
var repoName = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)

// RenderReadme renders the block for the GitHub profile README from doc, the
// data file of language lang, including both markers and a trailing newline.
// The same input yields the same bytes.
//
// The block is a summary, not the whole CV. It holds the experience,
// education, certificates, skills by category, projects and languages.
// docs/concepts/cv.md's "No visibility" settles that every committed entry is
// public, so nothing is filtered by entry. What the block leaves out is a
// choice of the format: the contact email and postal address, references,
// engagements and tools never appear, since a profile README is read by
// everyone and these sections add risk and no value there.
//
// RenderReadme does not validate doc; the caller does, through Load or
// LoadPair, before it renders.
func RenderReadme(doc Document, lang string) ([]byte, error) {
	if !slices.Contains(Languages, lang) {
		return nil, fmt.Errorf("cv: render readme: unknown language %q", lang)
	}

	var b strings.Builder
	b.WriteString(ReadmeBeginMarker + "\n")
	b.WriteString("<!-- Generated from ostendorf-cloud/cv. Edit the data there, not this block. -->\n")

	if len(doc.Experience) > 0 {
		b.WriteString("\n### Experience\n\n")
		for _, e := range doc.Experience {
			fmt.Fprintf(&b, "- %s, %s (%s)\n", oneLine(e.Title), oneLine(e.Employer), period(e.Start, e.End))
		}
	}
	if len(doc.Education) > 0 {
		b.WriteString("\n### Education\n\n")
		for _, e := range doc.Education {
			fmt.Fprintf(&b, "- %s, %s (%s)\n", oneLine(e.Degree), oneLine(e.Institution), period(e.Start, e.End))
		}
	}
	if len(doc.Certificates) > 0 {
		b.WriteString("\n### Certificates\n\n")
		for _, e := range doc.Certificates {
			fmt.Fprintf(&b, "- %s, %s (%s)\n", oneLine(e.Name), oneLine(e.Issuer), e.Issued)
		}
	}
	if section := skillLines(doc); section != "" {
		b.WriteString("\n### Skills\n\n" + section)
	}
	if len(doc.Projects) > 0 {
		b.WriteString("\n### Projects\n\n")
		for _, e := range doc.Projects {
			repo := oneLine(e.GithubRepo)
			if !repoName.MatchString(repo) {
				return nil, fmt.Errorf("cv: render readme: project %q: github_repo is not owner/name: %w", e.ID, ErrReadmeUnsafeContent)
			}
			fmt.Fprintf(&b, "- [%s](https://github.com/%s): %s\n", repo, repo, oneLine(e.Description))
		}
	}
	if len(doc.Languages) > 0 {
		b.WriteString("\n### Languages\n\n")
		for _, e := range doc.Languages {
			fmt.Fprintf(&b, "- %s (%s)\n", oneLine(e.Name), e.Level)
		}
	}
	b.WriteString(ReadmeEndMarker + "\n")

	out := b.String()
	// The two markers each occur exactly once: the ones written above.
	if strings.Count(out, ReadmeBeginMarker) != 1 || strings.Count(out, ReadmeEndMarker) != 1 {
		return nil, fmt.Errorf("cv: render readme: %w", ErrReadmeMarkerInContent)
	}
	// Besides the two markers the block holds one comment of its own. Any
	// further opener comes from CV text, and an unclosed one hides the rest of
	// the README on GitHub.
	if strings.Count(out, "<!--") != 3 {
		return nil, fmt.Errorf("cv: render readme: text contains an HTML comment opener: %w", ErrReadmeUnsafeContent)
	}
	return []byte(out), nil
}

// skillLines returns one line per non empty category, in display order, each
// listing the skill names in file order. A category without skills is omitted.
func skillLines(doc Document) string {
	var b strings.Builder
	for _, c := range sortedCategories(doc.SkillCategories) {
		var names []string
		for _, s := range doc.Skills {
			if s.Category == c.ID {
				names = append(names, oneLine(s.Name))
			}
		}
		if len(names) == 0 {
			continue
		}
		fmt.Fprintf(&b, "- **%s:** %s\n", oneLine(c.Name), strings.Join(names, ", "))
	}
	return b.String()
}

// period formats a start and an optional end. An empty end reads as "present".
func period(start, end string) string {
	if strings.TrimSpace(end) == "" {
		return start + " to present"
	}
	return start + " to " + end
}

// oneLine collapses every run of whitespace, newlines included, to one space,
// so a folded YAML text stays on one Markdown line.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// ReplaceReadmeBlock returns readme with the delimited block replaced by
// block, which RenderReadme produced. Every byte outside the two markers
// stays as it was. It refuses a README with a missing, duplicated or
// misordered marker and writes nothing in that case.
func ReplaceReadmeBlock(readme, block []byte) ([]byte, error) {
	begin, end := []byte(ReadmeBeginMarker), []byte(ReadmeEndMarker)

	for _, m := range []struct {
		name string
		text []byte
	}{{"begin", begin}, {"end", end}} {
		switch n := bytes.Count(readme, m.text); {
		case n == 0:
			return nil, fmt.Errorf("cv: readme %s marker %q: %w", m.name, m.text, ErrReadmeMarkerMissing)
		case n > 1:
			return nil, fmt.Errorf("cv: readme %s marker %q occurs %d times: %w", m.name, m.text, n, ErrReadmeMarkerDuplicate)
		}
	}

	from := bytes.Index(readme, begin)
	to := bytes.Index(readme, end)
	if to < from {
		return nil, fmt.Errorf("cv: readme end marker comes before the begin marker: %w", ErrReadmeMarkerOrder)
	}
	to += len(end)

	// The block ends in a newline of its own. The README keeps its own bytes
	// after the end marker, so that newline is not written twice.
	body := bytes.TrimSuffix(block, []byte("\n"))

	// The block has LF endings. A README whose marker lines end in CRLF gets
	// a CRLF block, so the file keeps one style.
	crlf := bytes.HasPrefix(readme[from+len(begin):], []byte("\r\n")) ||
		bytes.HasPrefix(readme[to:], []byte("\r\n"))
	if crlf {
		body = bytes.ReplaceAll(body, []byte("\n"), []byte("\r\n"))
	}

	out := make([]byte, 0, len(readme)-(to-from)+len(body))
	out = append(out, readme[:from]...)
	out = append(out, body...)
	out = append(out, readme[to:]...)
	return out, nil
}
