package cv

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// readmeDocument fills every section. The sections the block leaves out carry
// a sentinel, so a test can prove the sentinel never reaches the block.
func readmeDocument() Document {
	one := 1
	return Document{
		Experience: []ExperienceEntry{
			{ID: "exp-1", Employer: "Example Employer", Title: "Platform Engineer", Start: "2022-03"},
			{ID: "exp-2", Employer: "Old Employer", Title: "Admin", Start: "2018-01", End: "2022-02"},
		},
		Education: []EducationEntry{
			{ID: "edu-1", Institution: "Example University", Degree: "BSc Computer Science", Start: "2015-10", End: "2018-09"},
		},
		Certificates: []CertificateEntry{
			{ID: "cert-1", Name: "Example Certificate", Issuer: "Example Body", Issued: "2023-01"},
		},
		SkillCategories: []SkillCategoryEntry{
			{ID: "later", Name: "Later Group"},
			{ID: "first", Name: "First Group", Order: &one},
			{ID: "empty", Name: "Empty Group"},
		},
		Skills: []SkillEntry{
			{ID: "s1", Name: "Kubernetes", Level: LevelExpert, Category: "first"},
			{ID: "s2", Name: "Go", Level: LevelWorking, Category: "later"},
			{ID: "s3", Name: "OpenTofu", Level: LevelProficient, Category: "first"},
		},
		Projects: []ProjectEntry{
			{ID: "p1", GithubRepo: "example/repo", Description: "A folded\n  description.\n"},
		},
		Languages: []LanguageEntry{
			{ID: "l1", Name: "English", Level: LevelProficient},
		},

		// Never in the block.
		References: []ReferenceEntry{
			{ID: "r1", Name: "SENTINEL-REFERENCE-NAME", Role: "SENTINEL-REFERENCE-ROLE"},
		},
		Contact: Contact{
			Email:   "sentinel-email@example.com",
			Address: &Address{Street: "SENTINEL-STREET 1", City: "SENTINEL-CITY"},
			Profiles: []Profile{
				{ID: "gh", Platform: "GitHub", URL: "https://sentinel-profile.example.com"},
			},
		},
		Engagements: []EngagementEntry{
			{ID: "e1", Experience: "exp-1", Title: "SENTINEL-ENGAGEMENT", Summary: "SENTINEL-ENGAGEMENT-SUMMARY"},
		},
		Tools: []ToolEntry{
			{ID: "t1", Name: "SENTINEL-TOOL", Group: GroupCommandLine, Description: "SENTINEL-TOOL-DESCRIPTION"},
		},
		Volunteering: []VolunteeringEntry{
			{ID: "v1", Organisation: "SENTINEL-VOLUNTEERING", Role: "SENTINEL-VOLUNTEERING-ROLE", Start: "2020-01"},
		},
		Awards: []AwardEntry{
			{ID: "a1", Title: "SENTINEL-AWARD", Issuer: "SENTINEL-AWARD-ISSUER", Date: "2021-01", Summary: "SENTINEL-AWARD-SUMMARY"},
		},
		ExamResults: []ExamResultEntry{
			{ID: "x1", Name: "SENTINEL-EXAM", Score: "SENTINEL-SCORE", Date: "2017-01", Summary: "SENTINEL-EXAM-SUMMARY"},
		},
		Courses: []CourseEntry{
			{ID: "c1", Name: "SENTINEL-COURSE", Provider: "SENTINEL-COURSE-PROVIDER", Date: "2019-01"},
		},
		Publications: []PublicationEntry{
			{ID: "pu1", Title: "SENTINEL-PUBLICATION", Publisher: "SENTINEL-PUBLISHER", Date: "2020-01", URL: "https://sentinel-publication.example.org", Summary: "SENTINEL-PUBLICATION-SUMMARY"},
		},
		Patents: []PatentEntry{
			{ID: "pa1", Title: "SENTINEL-PATENT", Number: "SENTINEL-PATENT-NUMBER", Date: "2020-01", URL: "https://sentinel-patent.example.org", Summary: "SENTINEL-PATENT-SUMMARY"},
		},
		Organisations: []OrganisationEntry{
			{ID: "o1", Name: "SENTINEL-ORGANISATION", Role: "SENTINEL-ORGANISATION-ROLE", Start: "2019-01"},
		},
	}
}

func TestRenderReadme(t *testing.T) {
	t.Parallel()

	got, err := RenderReadme(readmeDocument(), "en")
	if err != nil {
		t.Fatalf("RenderReadme() error = %v", err)
	}
	text := string(got)

	t.Run("the block is delimited by both markers, once each", func(t *testing.T) {
		t.Parallel()

		if !strings.HasPrefix(text, ReadmeBeginMarker+"\n") || !strings.HasSuffix(text, ReadmeEndMarker+"\n") {
			t.Errorf("block does not start and end with the markers:\n%s", text)
		}
		if strings.Count(text, ReadmeBeginMarker) != 1 || strings.Count(text, ReadmeEndMarker) != 1 {
			t.Errorf("a marker does not occur exactly once:\n%s", text)
		}
	})

	t.Run("the sections the block carries", func(t *testing.T) {
		t.Parallel()

		for _, want := range []string{
			"- Platform Engineer, Example Employer (2022-03 to present)\n",
			"- Admin, Old Employer (2018-01 to 2022-02)\n",
			"- BSc Computer Science, Example University (2015-10 to 2018-09)\n",
			"- Example Certificate, Example Body (2023-01)\n",
			"- [example/repo](https://github.com/example/repo): A folded description.\n",
			"- English (proficient)\n",
		} {
			if !strings.Contains(text, want) {
				t.Errorf("block lacks %q:\n%s", want, text)
			}
		}
	})

	t.Run("skills follow the category display order and omit an empty category", func(t *testing.T) {
		t.Parallel()

		first := strings.Index(text, "- **First Group:** Kubernetes, OpenTofu\n")
		later := strings.Index(text, "- **Later Group:** Go\n")
		if first < 0 || later < 0 || first > later {
			t.Errorf("category lines wrong or out of order:\n%s", text)
		}
		if strings.Contains(text, "Empty Group") {
			t.Errorf("an empty category is listed:\n%s", text)
		}
	})

	t.Run("an empty section writes no heading", func(t *testing.T) {
		t.Parallel()

		out, err := RenderReadme(Document{}, "en")
		if err != nil {
			t.Fatal(err)
		}
		want := ReadmeBeginMarker + "\n" +
			"<!-- Generated from ostendorf-cloud/cv. Edit the data there, not this block. -->\n" +
			ReadmeEndMarker + "\n"
		if string(out) != want {
			t.Errorf("empty document block = %q, want %q", out, want)
		}
	})
}

// TestRenderReadmeKeepsPrivateSectionsOut proves a profile README, the most
// public sink there is, carries none of the sections the block leaves out. The
// model has no visibility field (docs/concepts/cv.md, "No visibility"), so the
// gate is the section list of the renderer.
func TestRenderReadmeKeepsPrivateSectionsOut(t *testing.T) {
	t.Parallel()

	got, err := RenderReadme(readmeDocument(), "en")
	if err != nil {
		t.Fatalf("RenderReadme() error = %v", err)
	}
	for _, banned := range []string{
		"SENTINEL", "sentinel", "example.com", "Example Street", "@",
	} {
		if strings.Contains(string(got), banned) {
			t.Errorf("block contains %q:\n%s", banned, got)
		}
	}
}

func TestRenderReadmeIsDeterministic(t *testing.T) {
	t.Parallel()

	a, errA := RenderReadme(readmeDocument(), "en")
	b, errB := RenderReadme(readmeDocument(), "en")
	if errA != nil || errB != nil {
		t.Fatalf("errors: %v, %v", errA, errB)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("two renders differ")
	}
}

func TestRenderReadmeRefusals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		doc     Document
		lang    string
		wantErr error
	}{
		{name: "unknown language", doc: Document{}, lang: "fr"},
		{
			name:    "text carrying the end marker",
			doc:     Document{Experience: []ExperienceEntry{{ID: "x", Employer: "A " + ReadmeEndMarker, Title: "T", Start: "2020-01"}}},
			lang:    "en",
			wantErr: ErrReadmeMarkerInContent,
		},
		{
			name:    "text carrying the begin marker",
			doc:     Document{Skills: []SkillEntry{{ID: "x", Name: ReadmeBeginMarker, Category: "c"}}, SkillCategories: []SkillCategoryEntry{{ID: "c", Name: "C"}}},
			lang:    "en",
			wantErr: ErrReadmeMarkerInContent,
		},
		{
			name:    "an unclosed comment opener in a project description",
			doc:     Document{Projects: []ProjectEntry{{ID: "p", GithubRepo: "example/repo", Description: "Fine <!-- hides the rest"}}},
			lang:    "en",
			wantErr: ErrReadmeUnsafeContent,
		},
		{
			name:    "a comment opener in a skill name",
			doc:     Document{Skills: []SkillEntry{{ID: "x", Name: "<!--", Category: "c"}}, SkillCategories: []SkillCategoryEntry{{ID: "c", Name: "C"}}},
			lang:    "en",
			wantErr: ErrReadmeUnsafeContent,
		},
		{
			name:    "a comment opener in a date",
			doc:     Document{Experience: []ExperienceEntry{{ID: "x", Employer: "E", Title: "T", Start: "<!--"}}},
			lang:    "en",
			wantErr: ErrReadmeUnsafeContent,
		},
		{
			name:    "a repository that is not owner/name",
			doc:     Document{Projects: []ProjectEntry{{ID: "p", GithubRepo: "example/repo/../x", Description: "d"}}},
			lang:    "en",
			wantErr: ErrReadmeUnsafeContent,
		},
		{
			name:    "a repository with a link closing character",
			doc:     Document{Projects: []ProjectEntry{{ID: "p", GithubRepo: "example/repo)x", Description: "d"}}},
			lang:    "en",
			wantErr: ErrReadmeUnsafeContent,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, err := RenderReadme(tt.doc, tt.lang)
			if err == nil {
				t.Fatalf("RenderReadme() = %q, want an error", out)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
			if out != nil {
				t.Errorf("output = %q, want none on error", out)
			}
		})
	}
}

func TestReplaceReadmeBlock(t *testing.T) {
	t.Parallel()

	block := []byte(ReadmeBeginMarker + "\nNEW\n" + ReadmeEndMarker + "\n")

	tests := []struct {
		name    string
		readme  string
		want    string
		wantErr error
	}{
		{
			name:   "text around the block stays byte for byte",
			readme: "# Hi \r\n\ttabs  and trailing  \n\n" + ReadmeBeginMarker + "\nOLD\n" + ReadmeEndMarker + "\n\nBye \xff\n",
			want:   "# Hi \r\n\ttabs  and trailing  \n\n" + ReadmeBeginMarker + "\nNEW\n" + ReadmeEndMarker + "\n\nBye \xff\n",
		},
		{
			name:   "an empty block between the markers",
			readme: ReadmeBeginMarker + ReadmeEndMarker,
			want:   ReadmeBeginMarker + "\nNEW\n" + ReadmeEndMarker,
		},
		{
			name:   "no newline after the end marker",
			readme: "a\n" + ReadmeBeginMarker + "\nOLD\n" + ReadmeEndMarker,
			want:   "a\n" + ReadmeBeginMarker + "\nNEW\n" + ReadmeEndMarker,
		},
		{
			name:   "a CRLF README gets a CRLF block",
			readme: "a\r\n" + ReadmeBeginMarker + "\r\nOLD\r\n" + ReadmeEndMarker + "\r\nb\r\n",
			want:   "a\r\n" + ReadmeBeginMarker + "\r\nNEW\r\n" + ReadmeEndMarker + "\r\nb\r\n",
		},
		{
			name:   "a CRLF after the end marker only",
			readme: ReadmeBeginMarker + "OLD" + ReadmeEndMarker + "\r\n",
			want:   ReadmeBeginMarker + "\r\nNEW\r\n" + ReadmeEndMarker + "\r\n",
		},
		{name: "no marker at all", readme: "# Hi\n", wantErr: ErrReadmeMarkerMissing},
		{name: "empty readme", readme: "", wantErr: ErrReadmeMarkerMissing},
		{name: "begin marker only", readme: ReadmeBeginMarker + "\n", wantErr: ErrReadmeMarkerMissing},
		{name: "end marker only", readme: ReadmeEndMarker + "\n", wantErr: ErrReadmeMarkerMissing},
		{
			name:    "two blocks",
			readme:  ReadmeBeginMarker + ReadmeEndMarker + "\n" + ReadmeBeginMarker + ReadmeEndMarker,
			wantErr: ErrReadmeMarkerDuplicate,
		},
		{
			name:    "a second begin marker",
			readme:  ReadmeBeginMarker + ReadmeBeginMarker + ReadmeEndMarker,
			wantErr: ErrReadmeMarkerDuplicate,
		},
		{
			name:    "a second end marker",
			readme:  ReadmeBeginMarker + ReadmeEndMarker + ReadmeEndMarker,
			wantErr: ErrReadmeMarkerDuplicate,
		},
		{
			name:    "end before begin",
			readme:  ReadmeEndMarker + "\n" + ReadmeBeginMarker + "\n",
			wantErr: ErrReadmeMarkerOrder,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ReplaceReadmeBlock([]byte(tt.readme), block)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				if got != nil {
					t.Errorf("output = %q, want none on error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ReplaceReadmeBlock() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestReplaceReadmeBlockIsIdempotent(t *testing.T) {
	t.Parallel()

	block, err := RenderReadme(readmeDocument(), "en")
	if err != nil {
		t.Fatal(err)
	}
	readme := []byte("intro\n\n" + ReadmeBeginMarker + "\nstale\n" + ReadmeEndMarker + "\n\noutro\n")

	once, err := ReplaceReadmeBlock(readme, block)
	if err != nil {
		t.Fatal(err)
	}
	twice, err := ReplaceReadmeBlock(once, block)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(once, twice) {
		t.Errorf("second replacement changed the README:\n%s\n---\n%s", once, twice)
	}
	if !bytes.HasPrefix(once, []byte("intro\n\n")) || !bytes.HasSuffix(once, []byte("\n\noutro\n")) {
		t.Errorf("text around the block changed:\n%s", once)
	}

	crlf := []byte("intro\r\n" + ReadmeBeginMarker + "\r\nstale\r\n" + ReadmeEndMarker + "\r\noutro\r\n")
	first, err := ReplaceReadmeBlock(crlf, block)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ReplaceReadmeBlock(first, block)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("second replacement changed the CRLF README")
	}
	if bytes.Contains(bytes.ReplaceAll(first, []byte("\r\n"), nil), []byte("\n")) {
		t.Errorf("a bare LF is left in the CRLF README:\n%q", first)
	}
}
