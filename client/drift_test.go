package client_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"testing"

	"github.com/ostendorf-cloud/cv/client"
	"github.com/ostendorf-cloud/cv/internal/cv"
	"github.com/ostendorf-cloud/cv/internal/github"
)

// These tests tie the public types to what internal/cv really renders. A
// document is decoded into client.Document and encoded again. If the two
// JSON values differ, the renderer and the client disagree: a key the client
// lacks is dropped on the way back, a key only the client has comes back as
// null or an empty list. Either fails here, so the two cannot drift
// silently.

func TestDocumentMatchesRenderer(t *testing.T) {
	t.Parallel()

	de, en, err := cv.LoadPair(filepath.Join("..", "data", "cv.de.yaml"), filepath.Join("..", "data", "cv.en.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		lang string
		doc  func(t *testing.T) []byte
	}{
		{"golden full en", "en", golden("full.en.golden.json")},
		{"golden full de", "de", golden("full.de.golden.json")},
		{"real data en without GitHub data", "en", render(en, "en", nil)},
		{"real data de with GitHub data", "de", render(de, "de", enrichmentFor(de))},
		{"real data en with GitHub data", "en", render(en, "en", enrichmentFor(en))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := tt.doc(t)

			var doc client.Document
			if err := json.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			if doc.SchemaVersion != cv.SchemaVersion || doc.Language != tt.lang {
				t.Errorf("schema_version %d language %q, want %d %q", doc.SchemaVersion, doc.Language, cv.SchemaVersion, tt.lang)
			}

			again, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if diff := firstDiff(t, data, again); diff != "" {
				t.Errorf("the client types and the renderer differ at %s", diff)
			}
		})
	}
}

// TestLegacyDocument decodes the golden files of the shape schema_version 1
// had before its later sections. They lack keys on purpose, so they cannot go
// through the round trip above. A missing key must decode, to an empty list.
func TestLegacyDocument(t *testing.T) {
	t.Parallel()

	for _, lang := range cv.Languages {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()
			var doc client.Document
			if err := json.Unmarshal(golden("legacy."+lang+".golden.json")(t), &doc); err != nil {
				t.Fatal(err)
			}
			if doc.Language != lang || len(doc.Experience) == 0 {
				t.Errorf("language %q, %d experience entries", doc.Language, len(doc.Experience))
			}
			if doc.Awards != nil || doc.Tools != nil {
				t.Errorf("awards %v, tools %v, want nil for keys the document lacks", doc.Awards, doc.Tools)
			}
		})
	}
}

// TestDocumentCarriesGitHubData keeps the drift test above from passing with
// no project data in it: the enriched documents must really carry the two
// keys, and the client must read them.
func TestDocumentCarriesGitHubData(t *testing.T) {
	t.Parallel()

	_, en, err := cv.LoadPair(filepath.Join("..", "data", "cv.de.yaml"), filepath.Join("..", "data", "cv.en.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(en.Projects) < 2 {
		t.Fatalf("%d projects in the real data, want at least 2", len(en.Projects))
	}
	data := render(en, "en", enrichmentFor(en))(t)

	var doc client.Document
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	first := doc.Projects[0]
	if len(first.Languages) != 2 || first.Languages[0].Name != "Go" || first.Languages[0].Bytes != 900 {
		t.Errorf("languages = %+v", first.Languages)
	}
	if r := first.LatestRelease; r == nil || r.Tag != "v1.0.0" || r.Name == nil || r.URL == nil || r.PublishedAt == nil {
		t.Errorf("latest_release = %+v", first.LatestRelease)
	}
	second := doc.Projects[1]
	if r := second.LatestRelease; r == nil || r.Name != nil {
		t.Errorf("latest_release = %+v, want a release with a null name", second.LatestRelease)
	}
}

// TestGetReadsRenderedDocument reads what the real renderer wrote through a
// server, so the client's supported schema version is checked against
// cv.SchemaVersion too.
func TestGetReadsRenderedDocument(t *testing.T) {
	t.Parallel()

	_, en, err := cv.LoadPair(filepath.Join("..", "data", "cv.de.yaml"), filepath.Join("..", "data", "cv.en.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	data := render(en, "en", enrichmentFor(en))(t)
	url := serve(t, respond(http.StatusOK, data))

	res, err := newClient(t, client.WithBaseURL(url)).Get(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Document == nil || len(res.Document.Projects) != len(en.Projects) {
		t.Errorf("document = %+v", res.Document)
	}
}

func golden(name string) func(*testing.T) []byte {
	return func(t *testing.T) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "internal", "cv", "testdata", "render", name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
}

func render(doc cv.Document, lang string, enr cv.Enrichment) func(*testing.T) []byte {
	return func(t *testing.T) []byte {
		t.Helper()
		data, err := cv.RenderJSONEnriched(doc, lang, enr)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
}

// enrichmentFor gives each project of doc GitHub data in one of four shapes:
// languages and a full release, languages and a release without a name,
// languages and no release, and an empty entry that adds no key.
func enrichmentFor(doc cv.Document) cv.Enrichment {
	enr := cv.Enrichment{}
	for i, p := range doc.Projects {
		langs := []github.Language{{Name: "Go", Bytes: 900}, {Name: "Shell", Bytes: 100}}
		switch i % 4 {
		case 0:
			enr[p.GithubRepo] = github.Info{Languages: langs, LatestRelease: &github.Release{
				Tag: "v1.0.0", Name: "First", URL: "https://example.com/r/1", PublishedAt: "2026-11-01T10:00:00Z",
			}}
		case 1:
			enr[p.GithubRepo] = github.Info{Languages: langs, LatestRelease: &github.Release{Tag: "v2.0.0"}}
		case 2:
			enr[p.GithubRepo] = github.Info{Languages: langs}
		default:
			enr[p.GithubRepo] = github.Info{}
		}
	}
	return enr
}

// firstDiff compares two JSON documents as values, so key order and spacing
// do not count. It returns the path of the first difference, or "".
func firstDiff(t *testing.T, a, b []byte) string {
	t.Helper()
	var av, bv any
	if err := json.NewDecoder(bytes.NewReader(a)).Decode(&av); err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(bytes.NewReader(b)).Decode(&bv); err != nil {
		t.Fatal(err)
	}
	return diffAt(av, bv, "$")
}

func diffAt(a, b any, path string) string {
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok {
			return path
		}
		keys := make([]string, 0, len(av)+len(bv))
		for k := range av {
			keys = append(keys, k)
		}
		for k := range bv {
			if _, ok := av[k]; !ok {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range slices.Compact(keys) {
			x, inA := av[k]
			y, inB := bv[k]
			switch {
			case inA && !inB:
				return fmt.Sprintf("%s.%s (only in the renderer output)", path, k)
			case !inA && inB:
				return fmt.Sprintf("%s.%s (only in the client types)", path, k)
			}
			if d := diffAt(x, y, path+"."+k); d != "" {
				return d
			}
		}
		return ""
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return path
		}
		for i := range av {
			if d := diffAt(av[i], bv[i], fmt.Sprintf("%s[%d]", path, i)); d != "" {
				return d
			}
		}
		return ""
	default:
		if !reflect.DeepEqual(a, b) {
			return path
		}
		return ""
	}
}
