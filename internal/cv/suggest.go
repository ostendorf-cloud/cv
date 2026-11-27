package cv

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"
)

// MissingSkill is a language GitHub reports for at least one project that no
// skill claims.
type MissingSkill struct {
	Language string
	// Projects are the ids of the projects that use it, in file order.
	Projects []string
}

// StaleEvidence is a skill that claims a language and names a project in its
// evidence that no longer uses it.
type StaleEvidence struct {
	SkillID   string
	SkillName string
	Project   string
}

// Suggestions is what the repositories say against what the skills claim.
// It is advice for the owner and is never applied to a data file.
type Suggestions struct {
	Missing []MissingSkill
	Stale   []StaleEvidence
}

// Empty reports whether there is nothing to suggest.
func (s Suggestions) Empty() bool { return len(s.Missing) == 0 && len(s.Stale) == 0 }

// SuggestSkills compares the languages GitHub reports for the projects with
// the skills of the document pair. A skill matches a language when its id or
// its name, in either language file, equals the language name ignoring case,
// spaces, dots and dashes. Only the enrichment of a project that was looked up
// takes part: a project without an entry is neither missing nor stale.
//
// A skill claims a language when it matches one that GitHub reports for any
// project. A skill whose name is no reported language claims nothing the
// repositories can contradict, so it is never stale. The detected language
// says a repository uses it and nothing about the level, so no level is ever
// suggested.
func SuggestSkills(de, en Document, enr Enrichment) Suggestions {
	// The projects are the same in both files, matched by id. The English
	// file is the source for ids and repositories.
	var sugg Suggestions

	skills := append(slices.Clone(en.Skills), de.Skills...)
	claimed := map[string]bool{}
	for _, s := range skills {
		claimed[foldName(s.ID)] = true
		claimed[foldName(s.Name)] = true
	}

	// Languages in order of first appearance, with their projects.
	var order []string
	users := map[string][]string{}
	spelling := map[string]string{}
	usedBy := map[string]map[string]bool{} // project id -> normalized languages
	for _, p := range en.Projects {
		info, ok := enr.For(p)
		if !ok {
			continue
		}
		usedBy[p.ID] = map[string]bool{}
		for _, l := range info.Languages {
			key := foldName(l.Name)
			if _, seen := spelling[key]; !seen {
				spelling[key] = l.Name
				order = append(order, key)
			}
			usedBy[p.ID][key] = true
			users[key] = append(users[key], p.ID)
		}
	}

	for _, key := range order {
		if !claimed[key] {
			sugg.Missing = append(sugg.Missing, MissingSkill{Language: spelling[key], Projects: users[key]})
		}
	}

	for _, s := range en.Skills {
		if _, reported := spelling[foldName(s.ID)]; !reported {
			if _, reported = spelling[foldName(s.Name)]; !reported {
				continue
			}
		}
		for _, ev := range s.Evidence {
			if ev.Type != EvidenceTypeProject {
				continue
			}
			langs, looked := usedBy[ev.Ref]
			if !looked || langs[foldName(s.ID)] || langs[foldName(s.Name)] {
				continue
			}
			sugg.Stale = append(sugg.Stale, StaleEvidence{SkillID: s.ID, SkillName: s.Name, Project: ev.Ref})
		}
	}
	return sugg
}

// foldName folds a language or skill name for comparison.
func foldName(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '.' || r == '-' || r == '_' {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}

// Write prints the suggestions as a diff against the skills section: a plus
// line for a skill to consider adding, a minus line for evidence to review.
// The owner edits the data file by hand.
func (s Suggestions) Write(w io.Writer) error {
	if s.Empty() {
		_, err := fmt.Fprintln(w, "cv: skills match the repositories, nothing to suggest")
		return err
	}
	var b strings.Builder
	b.WriteString("--- skills (as claimed in data/)\n+++ skills (as the repositories suggest)\n")
	if len(s.Missing) > 0 {
		b.WriteString("\n# Languages in use that no skill claims. Decide per line whether it\n")
		b.WriteString("# is a skill, and at which level. A language alone proves no level.\n")
		for _, m := range s.Missing {
			fmt.Fprintf(&b, "+ %s    used by %s\n", m.Language, strings.Join(m.Projects, ", "))
		}
	}
	if len(s.Stale) > 0 {
		b.WriteString("\n# Evidence naming a project that no longer uses the skill.\n")
		for _, e := range s.Stale {
			fmt.Fprintf(&b, "- %s: evidence project %s\n", e.SkillID, e.Project)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}
