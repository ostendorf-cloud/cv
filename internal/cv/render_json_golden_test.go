package cv

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update rewrites the golden files from the current renderer output:
//
//	go test ./internal/cv -run TestRenderJSONGolden -update
//
// Review the diff of testdata/render/*.golden.json before committing it: a
// change there is a change to the public contract, and may need a new
// SchemaVersion. See README.md.
var update = flag.Bool("update", false, "rewrite the golden files")

// goldenDocument is a fake document with all eight sections. Every optional
// field is set on one entry and unset on another, and each language carries
// its own text, so a mixup of languages shows too.
func goldenDocument(lang string) Document {
	doc := Document{
		Experience: []ExperienceEntry{
			{ID: "exp-1", Employer: "Example Employer " + lang, Title: "Engineer " + lang, Start: "2020-01", End: "2022-01", Summary: "Summary one " + lang + ".\n"},
			{ID: "exp-2", Employer: "Other Employer " + lang, Title: "Lead " + lang, Start: "2022-02"},
		},
		Education: []EducationEntry{
			{ID: "edu-1", Institution: "Example University " + lang, Degree: "Degree " + lang, Start: "2015-10", End: "2019-06", Summary: "Degree summary " + lang + ".\n"},
			{ID: "edu-2", Institution: "Other School " + lang, Degree: "Course " + lang, Start: "2019-09"},
		},
		Certificates: []CertificateEntry{
			{ID: "cert-1", Name: "Certificate " + lang, Issuer: "Example Body " + lang, Issued: "2023-01", ValidUntil: "2026-01"},
		},
		Skills: []SkillEntry{
			{ID: "skill-1", Name: "Kubernetes " + lang, Level: LevelExpert, Evidence: []Evidence{
				{Type: EvidenceTypeCertificate, Ref: "cert-1"},
				{Type: EvidenceTypeProject, Ref: "proj-1"},
			}},
			{ID: "skill-2", Name: "Go " + lang, Level: LevelWorking},
		},
		Projects: []ProjectEntry{
			{ID: "proj-1", GithubRepo: "example/repo", Description: "Project text " + lang + ".\n"},
		},
		References: []ReferenceEntry{
			{ID: "ref-1", Name: "Example Person", Role: "Manager " + lang},
		},
		Languages: []LanguageEntry{
			{ID: "lang-1", Name: "English " + lang, Level: LevelProficient},
			{ID: "lang-2", Name: "German " + lang, Level: LevelExpert},
		},
		Contact: Contact{
			Email: "contact@example.com",
			Profiles: []Profile{
				{ID: "github", Platform: "GitHub", URL: "https://github.com/example"},
			},
		},
	}
	// The address is set with both fields in one language and with only the
	// street in the other.
	if lang == "de" {
		doc.Contact.Address = &Address{Street: "Example Street 1", City: "Example City"}
	} else {
		doc.Contact.Address = &Address{Street: "Example Street 1"}
	}
	return doc
}

func TestRenderJSONGolden(t *testing.T) {
	t.Parallel()

	for _, lang := range Languages {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			got, err := RenderJSON(goldenDocument(lang), lang)
			if err != nil {
				t.Fatalf("RenderJSON() error = %v", err)
			}

			path := filepath.Join("testdata", "render", "full."+lang+".golden.json")
			if *update {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}

			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden file: %v (run with -update to create it)", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("output differs from %s, run with -update and review the diff\n--- got\n%s\n--- want\n%s", path, got, want)
			}
		})
	}
}

func TestRenderJSONTrimsTrailingWhitespace(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		summary string
		want    string
	}{
		"a folded yaml newline is trimmed":     {summary: "Text.\n", want: `"summary": "Text."`},
		"trailing spaces and tabs are trimmed": {summary: "Text. \t\r\n\n", want: `"summary": "Text."`},
		"inner newlines stay":                  {summary: "One.\nTwo.\n", want: `"summary": "One.\nTwo."`},
		"leading whitespace stays":             {summary: "  Text.", want: `"summary": "  Text."`},
		"whitespace only becomes null":         {summary: " \n", want: `"summary": null`},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			doc := Document{
				Experience: []ExperienceEntry{{ID: "e", Employer: "E", Title: "T", Start: "2020-01", Summary: tc.summary}},
				Contact:    Contact{Email: "contact@example.com"},
			}
			data, err := RenderJSON(doc, "en")
			if err != nil {
				t.Fatalf("RenderJSON() error = %v", err)
			}
			if !bytes.Contains(data, []byte(tc.want)) {
				t.Errorf("output lacks %s:\n%s", tc.want, data)
			}
		})
	}
}
