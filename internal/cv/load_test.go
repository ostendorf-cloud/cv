package cv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input   string
		wantErr bool
	}{
		"a minimal valid document parses": {
			input: "contact:\n  email: contact@example.com\n",
		},
		"malformed yaml fails": {
			input:   "contact: [this is not a mapping\n",
			wantErr: true,
		},
		"an unknown field fails": {
			input:   "contact:\n  email: contact@example.com\n  fax: \"0000\"\n",
			wantErr: true,
		},
		"an entry carrying a visibility field fails": {
			input:   "skills:\n  - id: skill-1\n    name: Kubernetes\n    level: expert\n    visibility: public\ncontact:\n  email: contact@example.com\n",
			wantErr: true,
		},
		"a missing required field fails": {
			input:   "experience:\n  - id: exp-1\n    employer: Acme\n    start: \"2020-01\"\ncontact:\n  email: contact@example.com\n",
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse("test", []byte(tc.input))
			if tc.wantErr && err == nil {
				t.Fatalf("Parse() error = nil, want an error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Parse() error = %v, want nil", err)
			}
		})
	}
}

// TestParseRejectsAVisibilityFieldAsUnknown proves that an entry still
// carrying the removed visibility field is rejected by the strict decoder
// naming that exact field, not silently ignored and not confused with some
// other problem.
func TestParseRejectsAVisibilityFieldAsUnknown(t *testing.T) {
	t.Parallel()

	input := "skills:\n  - id: skill-1\n    name: Kubernetes\n    level: expert\n    visibility: public\ncontact:\n  email: contact@example.com\n"
	_, err := Parse("test", []byte(input))
	if err == nil {
		t.Fatal("Parse() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "field visibility not found") {
		t.Fatalf("Parse() error = %q, want it to name the removed visibility field as unknown", err)
	}
}

// TestParseReportsDecodeAndValidationProblemsTogether proves that an
// unknown field (caught while decoding) and a missing required field
// (caught while validating) both surface from the same Parse call, rather
// than the decode error hiding the validation one.
func TestParseReportsDecodeAndValidationProblemsTogether(t *testing.T) {
	t.Parallel()

	input := `
skills:
  - id: skill-1
    level: expert
    bogus: true
`
	_, err := Parse("test", []byte(input))
	if err == nil {
		t.Fatal("Parse() error = nil, want an error")
	}

	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("Parse() error = %T, want a joined error", err)
	}
	// One problem from the strict decode (the unknown "bogus" field), one
	// from Validate (the missing "name" and the missing contact email):
	// at least two top level problems joined together.
	if got := len(joined.Unwrap()); got < 2 {
		t.Fatalf("Parse() reported %d top level problems, want at least 2: %v", got, err)
	}
}

func TestLoad(t *testing.T) {
	t.Parallel()

	if _, err := Load("testdata/does-not-exist.yaml"); err == nil {
		t.Fatal("Load() error = nil, want an error for a missing file")
	}
}

// TestLoadAgainstFixtures runs Load over every file under testdata/valid
// and testdata/invalid, each expected to pass or fail respectively.
func TestLoadAgainstFixtures(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) { checkFixtures(t, "testdata/valid", false) })
	t.Run("invalid", func(t *testing.T) { checkFixtures(t, "testdata/invalid", true) })
}

func checkFixtures(t *testing.T, dir string, wantErr bool) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("os.ReadDir(%q) error = %v", dir, err)
	}
	if len(entries) == 0 {
		t.Fatalf("%q has no fixtures", dir)
	}

	for _, entry := range entries {
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()

			_, err := Load(filepath.Join(dir, entry.Name()))
			if wantErr && err == nil {
				t.Fatalf("Load(%s) error = nil, want an error", entry.Name())
			}
			if !wantErr && err != nil {
				t.Fatalf("Load(%s) error = %v, want nil", entry.Name(), err)
			}
		})
	}
}

// TestLoadMultipleProblemsFixtureNamesEveryProblem proves that loading
// testdata/invalid/multiple-problems.yaml reports all four of its unrelated
// problems in the aggregated error, not just the one that happens to make
// TestLoadAgainstFixtures' directory walk see an error.
func TestLoadMultipleProblemsFixtureNamesEveryProblem(t *testing.T) {
	t.Parallel()

	_, err := Load("testdata/invalid/multiple-problems.yaml")
	if err == nil {
		t.Fatal("Load() error = nil, want an error")
	}

	// A missing name and an unknown level on the first skill, a reference to
	// a project id that does not exist on the second, and the id the two
	// share reported as a duplicate: four distinct problems, all present in
	// one error.
	want := []string{
		`skills[0] (id=kubernetes): name: is required`,
		`skills[0] (id=kubernetes): level: unknown value "guru"`,
		`skills[1] (id=kubernetes).evidence[0]: ref: no project with id "no-such-project" exists`,
		`skills: duplicate id "kubernetes"`,
	}
	got := err.Error()
	for _, substr := range want {
		if !strings.Contains(got, substr) {
			t.Errorf("Load() error = %q, want it to contain %q", got, substr)
		}
	}
}

func TestLoadPair(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		dir     string
		wantErr bool
	}{
		"a matched pair has no drift":      {dir: "testdata/pairs/matched"},
		"a drifted pair reports the drift": {dir: "testdata/pairs/drifted", wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, _, err := LoadPair(filepath.Join(tc.dir, "de.yaml"), filepath.Join(tc.dir, "en.yaml"))
			if tc.wantErr && err == nil {
				t.Fatalf("LoadPair() error = nil, want an error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("LoadPair() error = %v, want nil", err)
			}
		})
	}
}

// TestLoadPairReportsBothFilesTogether proves that a problem in the German
// file does not hide a problem in the English file: both load, both get
// validated, and both sets of problems come back from one call.
func TestLoadPairReportsBothFilesTogether(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	broken := "skills:\n  - id: skill-1\n    level: guru\ncontact:\n  email: a@example.com\n"
	if err := os.WriteFile(filepath.Join(dir, "de.yaml"), []byte(broken), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "en.yaml"), []byte(broken), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	_, _, err := LoadPair(filepath.Join(dir, "de.yaml"), filepath.Join(dir, "en.yaml"))
	if err == nil {
		t.Fatal("LoadPair() error = nil, want an error")
	}

	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("LoadPair() error = %T, want a joined error", err)
	}
	// Both files carry the same mistake: both must be reported, not only
	// the first file's.
	if got := len(joined.Unwrap()); got != 2 {
		t.Fatalf("LoadPair() reported %d top level problems, want 2: %v", got, err)
	}
}
