package cv

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// deepDocument is a fake document with every section filled, every optional
// field set, and nesting at its deepest: skill evidence inside skills, a
// profile list and an address inside contact.
func deepDocument() Document {
	return Document{
		Experience: []ExperienceEntry{
			{ID: "exp-1", Employer: "Example Employer", Title: "Engineer", Start: "2020-01", End: "2022-01", Summary: "First summary."},
			{ID: "exp-2", Employer: "Other Employer", Title: "Lead", Start: "2022-02"},
		},
		Education: []EducationEntry{
			{ID: "edu-1", Institution: "Example University", Degree: "Example Degree", Start: "2015-10", End: "2019-06", Summary: "Degree summary."},
		},
		Certificates: []CertificateEntry{
			{ID: "cert-1", Name: "Example Certificate", Issuer: "Example Body", Issued: "2023-01", ValidUntil: "2026-01"},
		},
		Skills: []SkillEntry{
			{ID: "skill-1", Name: "Kubernetes", Level: LevelExpert, Evidence: []Evidence{
				{Type: EvidenceTypeCertificate, Ref: "cert-1"},
				{Type: EvidenceTypeProject, Ref: "proj-1"},
				{Type: EvidenceTypeExperience, Ref: "exp-1"},
			}},
			{ID: "skill-2", Name: "Go", Level: LevelWorking},
		},
		Projects: []ProjectEntry{
			{ID: "proj-1", GithubRepo: "example/repo", Description: "A project."},
		},
		References: []ReferenceEntry{
			{ID: "ref-1", Name: "Example Person", Role: "Manager"},
		},
		Languages: []LanguageEntry{
			{ID: "lang-1", Name: "English", Level: LevelProficient},
		},
		Contact: Contact{
			Email:   "contact@example.com",
			Address: &Address{Street: "Example Street 1", City: "Example City"},
			Profiles: []Profile{
				{ID: "github", Platform: "GitHub", URL: "https://github.com/example"},
				{ID: "site", Platform: "Site", URL: "https://example.com"},
			},
		},
	}
}

func decode(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, data)
	}
	return out
}

// collectIDs walks any decoded JSON value and returns every string found
// under an "id" key, at any depth.
func collectIDs(v any, ids *[]string) {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if s, ok := val.(string); ok && k == "id" {
				*ids = append(*ids, s)
			}
			collectIDs(val, ids)
		}
	case []any:
		for _, val := range x {
			collectIDs(val, ids)
		}
	}
}

// hasKey reports whether any object at any depth has the key.
func hasKey(v any, key string) bool {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if k == key || hasKey(val, key) {
				return true
			}
		}
	case []any:
		for _, val := range x {
			if hasKey(val, key) {
				return true
			}
		}
	}
	return false
}

func TestRenderJSONSchemaVersion(t *testing.T) {
	t.Parallel()

	for _, lang := range Languages {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			data, err := RenderJSON(deepDocument(), lang)
			if err != nil {
				t.Fatalf("RenderJSON() error = %v", err)
			}
			got := decode(t, data)

			// A consumer decodes the version as a number, so a string
			// "1" or a missing key must fail here.
			version, ok := got["schema_version"].(float64)
			if !ok {
				t.Fatalf("schema_version = %#v, want a JSON number", got["schema_version"])
			}
			if int(version) != SchemaVersion || SchemaVersion != 1 {
				t.Errorf("schema_version = %v, SchemaVersion = %d, want both 1", version, SchemaVersion)
			}
			if got["language"] != lang {
				t.Errorf("language = %v, want %q", got["language"], lang)
			}
		})
	}
}

func TestRenderJSONKeyOrder(t *testing.T) {
	t.Parallel()

	data, err := RenderJSON(deepDocument(), "en")
	if err != nil {
		t.Fatalf("RenderJSON() error = %v", err)
	}

	want := []string{
		`"schema_version"`, `"language"`, `"experience"`, `"education"`, `"certificates"`,
		`"skills"`, `"projects"`, `"references"`, `"languages"`, `"contact"`,
	}
	last := -1
	for _, key := range want {
		i := bytes.Index(data, []byte("\n  "+key+":"))
		if i < 0 {
			t.Fatalf("top level key %s missing from\n%s", key, data)
		}
		if i < last {
			t.Errorf("top level key %s is out of order", key)
		}
		last = i
	}
	if !bytes.HasSuffix(data, []byte("}\n")) {
		t.Errorf("output does not end with a closing brace and one newline")
	}
	if !bytes.HasPrefix(data, []byte("{\n  \"schema_version\": 1,\n")) {
		t.Errorf("output does not start with the schema version on its own line:\n%s", data)
	}
}

// TestRenderJSONKeepsEveryEntry proves the contract docs/concepts/cv.md's
// "No visibility" states: nothing is filtered, so every id of the input, at
// any depth, reaches the output, and no visibility key exists anywhere.
func TestRenderJSONKeepsEveryEntry(t *testing.T) {
	t.Parallel()

	doc := deepDocument()
	data, err := RenderJSON(doc, "en")
	if err != nil {
		t.Fatalf("RenderJSON() error = %v", err)
	}
	got := decode(t, data)

	var gotIDs []string
	collectIDs(got, &gotIDs)
	have := map[string]bool{}
	for _, id := range gotIDs {
		have[id] = true
	}
	for _, id := range []string{
		"exp-1", "exp-2", "edu-1", "cert-1", "skill-1", "skill-2", "proj-1", "ref-1", "lang-1", "github", "site",
	} {
		if !have[id] {
			t.Errorf("id %q missing from the output", id)
		}
	}

	for _, key := range []string{"visibility", "cv-only", "public"} {
		if hasKey(got, key) {
			t.Errorf("output carries the key %q", key)
		}
	}
	if strings.Contains(string(data), "cv-only") {
		t.Errorf("output mentions cv-only")
	}

	var typed struct {
		Skills []struct {
			Evidence []any `json:"evidence"`
		} `json:"skills"`
		Contact struct {
			Profiles []any `json:"profiles"`
		} `json:"contact"`
	}
	if err := json.Unmarshal(data, &typed); err != nil {
		t.Fatal(err)
	}
	if len(typed.Skills) != 2 || len(typed.Skills[0].Evidence) != 3 {
		t.Errorf("skills = %+v, want 2 skills and 3 evidence entries on the first", typed.Skills)
	}
	if len(typed.Contact.Profiles) != 2 {
		t.Errorf("profiles length = %d, want 2", len(typed.Contact.Profiles))
	}
}

// TestModelHasNoVisibility guards the assumption RenderJSON rests on: the
// data model carries no visibility field at any depth. If one returns, the
// renderer needs a filter and a test proving no hidden entry reaches the
// output, and this test fails until that exists.
func TestModelHasNoVisibility(t *testing.T) {
	t.Parallel()

	var walk func(t reflect.Type, path string)
	seen := map[reflect.Type]bool{}
	walk = func(typ reflect.Type, path string) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || seen[typ] {
			return
		}
		seen[typ] = true
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			name := strings.ToLower(f.Name + " " + f.Tag.Get("yaml"))
			if strings.Contains(name, "visib") {
				t.Errorf("%s.%s looks like a visibility field: add a filter and a depth test to RenderJSON", path, f.Name)
			}
			walk(f.Type, path+"."+f.Name)
		}
	}
	walk(reflect.TypeOf(Document{}), "Document")
}

func TestRenderJSONDeterministic(t *testing.T) {
	t.Parallel()

	first, err := RenderJSON(deepDocument(), "de")
	if err != nil {
		t.Fatalf("RenderJSON() error = %v", err)
	}
	for i := 0; i < 50; i++ {
		again, err := RenderJSON(deepDocument(), "de")
		if err != nil {
			t.Fatalf("RenderJSON() error = %v", err)
		}
		if !bytes.Equal(first, again) {
			t.Fatalf("run %d differs from the first run:\n%s\n---\n%s", i, first, again)
		}
	}
}

func TestRenderJSONShapes(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		doc  Document
		want []string
	}{
		"an empty document renders empty lists, not null": {
			doc: Document{Contact: Contact{Email: "contact@example.com"}},
			want: []string{
				`"experience": []`, `"education": []`, `"certificates": []`, `"skills": []`,
				`"projects": []`, `"references": []`, `"languages": []`, `"profiles": []`,
				`"address": null`,
			},
		},
		"an unset optional field renders null": {
			doc: Document{
				Experience: []ExperienceEntry{{ID: "exp-1", Employer: "E", Title: "T", Start: "2020-01"}},
				Contact:    Contact{Email: "contact@example.com"},
			},
			want: []string{`"end": null`, `"summary": null`},
		},
		"a skill without evidence renders an empty list": {
			doc: Document{
				Skills:  []SkillEntry{{ID: "s", Name: "Go", Level: LevelWorking}},
				Contact: Contact{Email: "contact@example.com"},
			},
			want: []string{`"evidence": []`},
		},
		"html characters stay unescaped": {
			doc: Document{
				Projects: []ProjectEntry{{ID: "p", GithubRepo: "a/b", Description: "R&D <tools>"}},
				Contact:  Contact{Email: "contact@example.com"},
			},
			want: []string{`"description": "R&D <tools>"`},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			data, err := RenderJSON(tc.doc, "en")
			if err != nil {
				t.Fatalf("RenderJSON() error = %v", err)
			}
			for _, want := range tc.want {
				if !strings.Contains(string(data), want) {
					t.Errorf("output lacks %s:\n%s", want, data)
				}
			}
		})
	}
}

func TestRenderJSONLanguageIsOwn(t *testing.T) {
	t.Parallel()

	de := Document{Skills: []SkillEntry{{ID: "s", Name: "Kubernetes DE", Level: LevelExpert}}, Contact: Contact{Email: "a@example.com"}}
	en := Document{Skills: []SkillEntry{{ID: "s", Name: "Kubernetes EN", Level: LevelExpert}}, Contact: Contact{Email: "a@example.com"}}

	deOut, err := RenderJSON(de, "de")
	if err != nil {
		t.Fatalf("RenderJSON(de) error = %v", err)
	}
	enOut, err := RenderJSON(en, "en")
	if err != nil {
		t.Fatalf("RenderJSON(en) error = %v", err)
	}
	if !strings.Contains(string(deOut), "Kubernetes DE") || strings.Contains(string(deOut), "Kubernetes EN") {
		t.Errorf("the de document mixes languages:\n%s", deOut)
	}
	if !strings.Contains(string(enOut), "Kubernetes EN") || strings.Contains(string(enOut), "Kubernetes DE") {
		t.Errorf("the en document mixes languages:\n%s", enOut)
	}
}

func TestRenderJSONUnknownLanguage(t *testing.T) {
	t.Parallel()

	if _, err := RenderJSON(deepDocument(), "fr"); err == nil {
		t.Fatal("RenderJSON(fr) error = nil, want an error")
	}
}

func TestJSONFileName(t *testing.T) {
	t.Parallel()

	tests := map[string]string{"de": "cv.de.json", "en": "cv.en.json"}
	for lang, want := range tests {
		if got := JSONFileName(lang); got != want {
			t.Errorf("JSONFileName(%q) = %q, want %q", lang, got, want)
		}
	}
}
