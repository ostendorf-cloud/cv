package cv

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/ostendorf-cloud/cv/internal/github"
)

func langs(names ...string) github.Info {
	var info github.Info
	for _, n := range names {
		info.Languages = append(info.Languages, github.Language{Name: n, Bytes: 1})
	}
	return info
}

func TestSuggestSkills(t *testing.T) {
	t.Parallel()

	projects := []ProjectEntry{
		{ID: "alpha", GithubRepo: "example/alpha"},
		{ID: "beta", GithubRepo: "example/beta"},
	}
	goSkill := SkillEntry{
		ID: "go", Name: "Go",
		Evidence: []Evidence{{Type: EvidenceTypeProject, Ref: "alpha"}, {Type: EvidenceTypeProject, Ref: "beta"}},
	}

	tests := []struct {
		name        string
		skills      []SkillEntry
		enr         Enrichment
		wantMissing []MissingSkill
		wantStale   []StaleEvidence
	}{
		{
			name:   "language no skill claims",
			skills: []SkillEntry{goSkill},
			enr: Enrichment{
				"example/alpha": langs("Go", "Rust"),
				"example/beta":  langs("Go", "Rust"),
			},
			wantMissing: []MissingSkill{{Language: "Rust", Projects: []string{"alpha", "beta"}}},
		},
		{
			name:   "skill matches by name ignoring case and punctuation",
			skills: []SkillEntry{{ID: "nodejs", Name: "Node.js"}},
			enr:    Enrichment{"example/alpha": langs("node.JS")},
		},
		{
			name:   "skill matches by id",
			skills: []SkillEntry{{ID: "shell", Name: "Shell scripting"}},
			enr:    Enrichment{"example/alpha": langs("Shell")},
		},
		{
			name:   "evidence project dropped the language",
			skills: []SkillEntry{goSkill},
			enr: Enrichment{
				"example/alpha": langs("Go"),
				"example/beta":  langs("Python"),
			},
			wantMissing: []MissingSkill{{Language: "Python", Projects: []string{"beta"}}},
			wantStale:   []StaleEvidence{{SkillID: "go", SkillName: "Go", Project: "beta"}},
		},
		{
			name:   "project without lookup is neither missing nor stale",
			skills: []SkillEntry{goSkill},
			enr:    Enrichment{"example/alpha": langs("Go")},
		},
		{
			name: "evidence of another type is not compared",
			skills: []SkillEntry{{
				ID: "go", Name: "Go",
				Evidence: []Evidence{{Type: EvidenceTypeExperience, Ref: "job"}},
			}},
			enr: Enrichment{"example/alpha": langs("Go")},
		},
		{
			name:   "skill that is no reported language is never stale",
			skills: []SkillEntry{{ID: "web", Name: "Web development", Evidence: []Evidence{{Type: EvidenceTypeProject, Ref: "alpha"}}}},
			enr:    Enrichment{"example/alpha": langs("Go")},
			wantMissing: []MissingSkill{
				{Language: "Go", Projects: []string{"alpha"}},
			},
		},
		{
			name: "nil enrichment",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := Document{Projects: projects, Skills: tt.skills}
			got := SuggestSkills(doc, doc, tt.enr)
			if !slices.EqualFunc(got.Missing, tt.wantMissing, func(a, b MissingSkill) bool {
				return a.Language == b.Language && slices.Equal(a.Projects, b.Projects)
			}) {
				t.Errorf("Missing = %+v, want %+v", got.Missing, tt.wantMissing)
			}
			if !slices.Equal(got.Stale, tt.wantStale) {
				t.Errorf("Stale = %+v, want %+v", got.Stale, tt.wantStale)
			}
		})
	}
}

func TestSuggestSkillsUsesBothLanguageFiles(t *testing.T) {
	t.Parallel()

	de := Document{Skills: []SkillEntry{{ID: "rust", Name: "Rust"}}}
	en := Document{Projects: []ProjectEntry{{ID: "alpha", GithubRepo: "example/alpha"}}}
	got := SuggestSkills(de, en, Enrichment{"example/alpha": langs("Rust")})
	if !got.Empty() {
		t.Errorf("a skill claimed in the other file was suggested: %+v", got)
	}
}

func TestSuggestions_Write(t *testing.T) {
	t.Parallel()

	empties := []struct {
		name string
		s    Suggestions
		want string
	}{
		{"compared, nothing to suggest", Suggestions{Compared: 2}, "nothing to suggest"},
		{"nothing compared", Suggestions{}, "nothing was compared"},
	}
	for _, tt := range empties {
		var empty bytes.Buffer
		if err := tt.s.Write(&empty); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(empty.String(), tt.want) {
			t.Errorf("%s: output = %q, want %q", tt.name, empty.String(), tt.want)
		}
	}

	var b bytes.Buffer
	s := Suggestions{
		Missing: []MissingSkill{{Language: "Rust", Projects: []string{"alpha", "beta"}}},
		Stale:   []StaleEvidence{{SkillID: "go", SkillName: "Go", Project: "beta"}},
	}
	if err := s.Write(&b); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"+ Rust    used by alpha, beta\n", "- go: evidence project beta\n"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, b.String())
		}
	}
}
