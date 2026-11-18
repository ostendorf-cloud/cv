package cv

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
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

// pad surrounds every value of the padded fixture, including the unset
// optional ones, with whitespace the renderer has to remove again.
func pad(s string) string { return " \t" + s + " \n\n" }

func plain(s string) string { return s }

// fixtureDocument is a fake document with all eight sections. Every optional
// field is set on one entry and unset on another, in each language: the
// address carries a street and a city in de and a city only in en. Each
// language carries its own text, so a mixup of languages shows too. Every
// string goes through p, which is plain for the golden fixture and pad for
// the whitespace fixture.
func fixtureDocument(lang string, p func(string) string) Document {
	doc := Document{
		Experience: []ExperienceEntry{
			{ID: p("exp-1"), Employer: p("Example Employer " + lang), Title: p("Engineer " + lang), Start: p("2020-01"), End: p("2022-01"), Summary: p("Summary one " + lang + ".")},
			{ID: p("exp-2"), Employer: p("Other Employer " + lang), Title: p("Lead " + lang), Start: p("2022-02"), End: p(""), Summary: p("")},
		},
		Education: []EducationEntry{
			{ID: p("edu-1"), Institution: p("Example University " + lang), Degree: p("Degree " + lang), Start: p("2015-10"), End: p("2019-06"), Summary: p("Degree summary " + lang + ".")},
			{ID: p("edu-2"), Institution: p("Other School " + lang), Degree: p("Course " + lang), Start: p("2019-09"), End: p(""), Summary: p("")},
		},
		Certificates: []CertificateEntry{
			{ID: p("cert-1"), Name: p("Certificate " + lang), Issuer: p("Example Body " + lang), Issued: p("2023-01"), ValidUntil: p("2026-01")},
		},
		Skills: []SkillEntry{
			{ID: p("skill-1"), Name: p("Kubernetes " + lang), Level: Level(p(string(LevelExpert))), Evidence: []Evidence{
				{Type: EvidenceType(p(string(EvidenceTypeCertificate))), Ref: p("cert-1")},
				{Type: EvidenceType(p(string(EvidenceTypeProject))), Ref: p("proj-1")},
			}},
			{ID: p("skill-2"), Name: p("Go " + lang), Level: Level(p(string(LevelWorking)))},
		},
		Projects: []ProjectEntry{
			{ID: p("proj-1"), GithubRepo: p("example/repo"), Description: p("Project text " + lang + ".")},
		},
		References: []ReferenceEntry{
			{ID: p("ref-1"), Name: p("Example Person"), Role: p("Manager " + lang)},
		},
		Languages: []LanguageEntry{
			{ID: p("lang-1"), Name: p("English " + lang), Level: Level(p(string(LevelProficient)))},
			{ID: p("lang-2"), Name: p("German " + lang), Level: Level(p(string(LevelExpert)))},
		},
		Contact: Contact{
			Email: p("contact@example.com"),
			Profiles: []Profile{
				{ID: p("github"), Platform: p("GitHub"), URL: p("https://github.com/example")},
			},
		},
	}
	if lang == "de" {
		doc.Contact.Address = &Address{Street: p("Example Street 1"), City: p("Example City")}
	} else {
		doc.Contact.Address = &Address{Street: p(""), City: p("Example City")}
	}
	return doc
}

func goldenDocument(lang string) Document { return fixtureDocument(lang, plain) }

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

// TestRenderJSONTrimsEveryField proves the contract: every text field has its
// leading and trailing whitespace removed, and a whitespace only optional
// value is null. The fixture carries padding in every string field, so the
// output must be byte for byte the golden file of the unpadded fixture.
func TestRenderJSONTrimsEveryField(t *testing.T) {
	t.Parallel()

	for _, lang := range Languages {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			padded, err := RenderJSON(fixtureDocument(lang, pad), lang)
			if err != nil {
				t.Fatalf("RenderJSON() error = %v", err)
			}
			want, err := os.ReadFile(filepath.Join("testdata", "render", "full."+lang+".golden.json"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(padded, want) {
				t.Errorf("padded input does not render as the golden file\n--- got\n%s\n--- want\n%s", padded, want)
			}
		})
	}
}

func TestRenderJSONSummaryWhitespace(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		summary string
		want    string
	}{
		"a folded yaml newline is trimmed":     {summary: "Text.\n", want: `"summary": "Text."`},
		"trailing spaces and tabs are trimmed": {summary: "Text. \t\r\n\n", want: `"summary": "Text."`},
		"leading whitespace is trimmed":        {summary: "  Text.", want: `"summary": "Text."`},
		"inner newlines stay":                  {summary: "One.\nTwo.\n", want: `"summary": "One.\nTwo."`},
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
			if !strings.Contains(string(data), tc.want) {
				t.Errorf("output lacks %s:\n%s", tc.want, data)
			}
		})
	}
}

// TestRenderJSONDoesNotChangeItsInput guards the trim: it must work on the
// output, never on the caller's document.
func TestRenderJSONDoesNotChangeItsInput(t *testing.T) {
	t.Parallel()

	doc := fixtureDocument("en", pad)
	if _, err := RenderJSON(doc, "en"); err != nil {
		t.Fatalf("RenderJSON() error = %v", err)
	}
	if got, want := doc.Experience[0].Employer, pad("Example Employer en"); got != want {
		t.Errorf("Employer = %q, want %q", got, want)
	}
	if got, want := doc.Skills[0].Evidence[0].Ref, pad("cert-1"); got != want {
		t.Errorf("Evidence ref = %q, want %q", got, want)
	}
}
