package cv

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTopLevelKeys are the sections added after schema_version 1 first
// shipped. newEntryKeys are the fields added to existing sections' entries.
var (
	newTopLevelKeys = []string{"volunteering", "awards", "exam_results", "courses", "publications", "patents", "organisations", "engagements", "skill_categories"}
	newEntryKeys    = map[string][]string{
		"skills":     {"category"},
		"experience": {"location", "employment_type"},
		"education":  {"grade"},
	}
)

// objectKeys decodes one JSON object into its keys in document order and
// their raw values.
func objectKeys(t *testing.T, raw []byte) ([]string, []json.RawMessage) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		t.Fatalf("not an object: %v %v", tok, err)
	}
	var keys []string
	var vals []json.RawMessage
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			t.Fatal(err)
		}
		key, ok := tok.(string)
		if !ok {
			t.Fatalf("key token = %v", tok)
		}
		keys = append(keys, key)
		vals = append(vals, v)
	}
	return keys, vals
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// stripNewKeys removes every key and section the LinkedIn parity change
// added, keeping the order and bytes of everything else.
func stripNewKeys(t *testing.T, data []byte) []byte {
	t.Helper()
	keys, vals := objectKeys(t, data)
	var out bytes.Buffer
	out.WriteByte('{')
	first := true
	for i, k := range keys {
		if contains(newTopLevelKeys, k) {
			continue
		}
		val := vals[i]
		if drop, ok := newEntryKeys[k]; ok {
			var entries []json.RawMessage
			if err := json.Unmarshal(val, &entries); err != nil {
				t.Fatal(err)
			}
			var rebuilt []string
			for _, e := range entries {
				ek, ev := objectKeys(t, e)
				var parts []string
				for j, key := range ek {
					if contains(drop, key) {
						continue
					}
					kb, _ := json.Marshal(key)
					parts = append(parts, string(kb)+":"+string(ev[j]))
				}
				rebuilt = append(rebuilt, "{"+strings.Join(parts, ",")+"}")
			}
			val = json.RawMessage("[" + strings.Join(rebuilt, ",") + "]")
		}
		if !first {
			out.WriteByte(',')
		}
		first = false
		kb, _ := json.Marshal(k)
		out.Write(kb)
		out.WriteByte(':')
		out.Write(val)
	}
	out.WriteByte('}')

	var compact, indented bytes.Buffer
	if err := json.Compact(&compact, out.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := json.Indent(&indented, compact.Bytes(), "", "  "); err != nil {
		t.Fatal(err)
	}
	indented.WriteByte('\n')
	return indented.Bytes()
}

// TestRenderJSONLegacyKeysUnchanged proves the existing part of the contract
// did not move. The old fixture renders, the new keys are cut out, and what
// is left must equal the golden file of the release before the new keys
// existed, byte for byte. Only appended keys may differ.
func TestRenderJSONLegacyKeysUnchanged(t *testing.T) {
	t.Parallel()

	for _, lang := range Languages {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			got, err := RenderJSON(legacyDocument(lang), lang)
			if err != nil {
				t.Fatalf("RenderJSON() error = %v", err)
			}
			got = stripNewKeys(t, got)

			path := filepath.Join("testdata", "render", "legacy."+lang+".golden.json")
			// No -update branch: the legacy goldens are frozen on purpose.
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read legacy golden: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("old keys changed\n--- got\n%s\n--- want\n%s", got, want)
			}
		})
	}
}

// legacyGoldenBlobs are the git blob sha1 hashes of the two legacy goldens,
// which equal the full goldens of the first release shape. A change to a
// legacy golden fails here even when someone regenerates it.
var legacyGoldenBlobs = map[string]string{
	"de": "ad406fab4c20c25df1d2fd2fd0bb5a0822ca7160",
	"en": "f8d4ea7b5e1d21a9be94a8680a071b3eca10a49e",
}

func TestLegacyGoldensAreFrozen(t *testing.T) {
	t.Parallel()

	for lang, want := range legacyGoldenBlobs {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(filepath.Join("testdata", "render", "legacy."+lang+".golden.json"))
			if err != nil {
				t.Fatal(err)
			}
			h := sha1.New() //nolint:gosec // git blob id, not a security use
			fmt.Fprintf(h, "blob %d\x00", len(data))
			h.Write(data)
			if got := hex.EncodeToString(h.Sum(nil)); got != want {
				t.Errorf("legacy golden %s blob = %s, want %s: the legacy goldens must never change", lang, got, want)
			}
		})
	}
}

// TestRenderJSONNewKeysComeLast checks the final top level key order and
// that the new sections follow contact.
func TestRenderJSONNewKeysComeLast(t *testing.T) {
	t.Parallel()

	data, err := RenderJSON(goldenDocument("en"), "en")
	if err != nil {
		t.Fatalf("RenderJSON() error = %v", err)
	}
	keys, _ := objectKeys(t, data)
	want := []string{
		"schema_version", "language", "experience", "education", "certificates", "skills",
		"projects", "references", "languages", "contact",
		"volunteering", "awards", "exam_results", "courses", "publications", "patents", "organisations", "engagements",
		"skill_categories",
	}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Errorf("top level keys = %v, want %v", keys, want)
	}
}

// TestRenderJSONNewFields covers every new section and field, set and unset.
func TestRenderJSONNewFields(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		doc  Document
		want []string
	}{
		"an empty document renders every new section as an empty list": {
			doc: Document{Contact: Contact{Email: "contact@example.com"}},
			want: []string{
				`"volunteering": []`, `"awards": []`, `"exam_results": []`, `"courses": []`,
				`"publications": []`, `"patents": []`, `"organisations": []`, `"engagements": []`,
				`"skill_categories": []`,
			},
		},
		"unset optional fields render null": {
			doc: Document{
				Experience:    []ExperienceEntry{{ID: "e", Employer: "E", Title: "T", Start: "2020-01"}},
				Education:     []EducationEntry{{ID: "d", Institution: "I", Degree: "D", Start: "2020-01"}},
				Volunteering:  []VolunteeringEntry{{ID: "v", Organisation: "O", Role: "R", Start: "2020-01"}},
				Awards:        []AwardEntry{{ID: "a", Title: "T", Issuer: "I", Date: "2020-01"}},
				ExamResults:   []ExamResultEntry{{ID: "x", Name: "N", Score: "1", Date: "2020-01"}},
				Courses:       []CourseEntry{{ID: "c", Name: "N"}},
				Publications:  []PublicationEntry{{ID: "p", Title: "T", Date: "2020-01"}},
				Patents:       []PatentEntry{{ID: "t", Title: "T", Date: "2020-01"}},
				Organisations: []OrganisationEntry{{ID: "o", Name: "N", Start: "2020-01"}},
				Engagements:   []EngagementEntry{{ID: "g", Experience: "e", Title: "T", Summary: "S"}},
				Contact:       Contact{Email: "contact@example.com"},
			},
			want: []string{
				`"industry": null`, `"location": null`, `"employment_type": null`, `"grade": null`, `"cause": null`,
				`"provider": null`, `"publisher": null`, `"number": null`, `"url": null`, `"role": null`,
			},
		},
		"set fields render their value": {
			doc: Document{
				Experience: []ExperienceEntry{{ID: "e", Employer: "E", Title: "T", Start: "2020-01", Location: "Berlin", EmploymentType: "Full-time"}},
				Education:  []EducationEntry{{ID: "d", Institution: "I", Degree: "D", Start: "2020-01", Grade: "0,0"}},
				Contact:    Contact{Email: "contact@example.com"},
			},
			want: []string{`"location": "Berlin"`, `"employment_type": "Full-time"`, `"grade": "0,0"`},
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

// TestRenderJSONTrimsNewTextFields checks that every new optional text field
// drops trailing whitespace and that a whitespace only value is null.
func TestRenderJSONTrimsNewTextFields(t *testing.T) {
	t.Parallel()

	const dirty = "Text.\n"
	tests := map[string]struct {
		doc  Document
		want string
	}{
		"experience location":      {Document{Experience: []ExperienceEntry{{ID: "e", Location: dirty}}}, `"location": "Text."`},
		"experience type":          {Document{Experience: []ExperienceEntry{{ID: "e", EmploymentType: dirty}}}, `"employment_type": "Text."`},
		"education grade":          {Document{Education: []EducationEntry{{ID: "e", Grade: dirty}}}, `"grade": "Text."`},
		"volunteering cause":       {Document{Volunteering: []VolunteeringEntry{{ID: "e", Cause: dirty}}}, `"cause": "Text."`},
		"volunteering summary":     {Document{Volunteering: []VolunteeringEntry{{ID: "e", Summary: dirty}}}, `"summary": "Text."`},
		"awards summary":           {Document{Awards: []AwardEntry{{ID: "e", Summary: dirty}}}, `"summary": "Text."`},
		"exam results summary":     {Document{ExamResults: []ExamResultEntry{{ID: "e", Summary: dirty}}}, `"summary": "Text."`},
		"courses provider":         {Document{Courses: []CourseEntry{{ID: "e", Provider: dirty}}}, `"provider": "Text."`},
		"publications summary":     {Document{Publications: []PublicationEntry{{ID: "e", Summary: dirty}}}, `"summary": "Text."`},
		"patents summary":          {Document{Patents: []PatentEntry{{ID: "e", Summary: dirty}}}, `"summary": "Text."`},
		"organisations role":       {Document{Organisations: []OrganisationEntry{{ID: "e", Role: dirty}}}, `"role": "Text."`},
		"engagements industry":     {Document{Engagements: []EngagementEntry{{ID: "e", Industry: dirty}}}, `"industry": "Text."`},
		"engagements summary":      {Document{Engagements: []EngagementEntry{{ID: "e", Summary: dirty}}}, `"summary": "Text."`},
		"whitespace only is null":  {Document{Education: []EducationEntry{{ID: "e", Grade: " \n"}}}, `"grade": null`},
		"whitespace only location": {Document{Experience: []ExperienceEntry{{ID: "e", Location: "\t"}}}, `"location": null`},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			data, err := RenderJSON(tc.doc, "en")
			if err != nil {
				t.Fatalf("RenderJSON() error = %v", err)
			}
			if !bytes.Contains(data, []byte(tc.want)) {
				t.Errorf("output lacks %s:\n%s", tc.want, data)
			}
		})
	}
}
