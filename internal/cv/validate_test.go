package cv

import "testing"

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		doc     Document
		wantErr bool
	}{
		"a fully populated document passes": {
			doc: Document{
				Experience: []ExperienceEntry{{
					ID: "exp-1", Employer: "Acme", Title: "Engineer", Start: "2020-01", Visibility: VisibilityPublic,
				}},
				Education: []EducationEntry{{
					ID: "edu-1", Institution: "University", Degree: "BSc", Start: "2016-10", Visibility: VisibilityPublic,
				}},
				Certificates: []CertificateEntry{{
					ID: "cert-1", Name: "CKAD", Issuer: "Linux Foundation", Issued: "2024-05", ValidUntil: "2027-05", Visibility: VisibilityPublic,
				}},
				Skills: []SkillEntry{{
					ID: "skill-1", Name: "Kubernetes", Level: LevelExpert, Visibility: VisibilityPublic,
					Evidence: []Evidence{
						{Type: EvidenceTypeProject, Ref: "proj-1"},
						{Type: EvidenceTypeExperience, Ref: "exp-1"},
						{Type: EvidenceTypeCertificate, Ref: "cert-1"},
					},
				}},
				Projects: []ProjectEntry{{
					ID: "proj-1", GithubRepo: "acme/cv", Description: "A CV", Visibility: VisibilityPublic,
				}},
				References: []ReferenceEntry{{
					ID: "ref-1", Name: "Jane Doe", Role: "Manager", Visibility: VisibilityCVOnly,
				}},
				Languages: []LanguageEntry{{
					ID: "lang-1", Name: "German", Level: LevelExpert, Visibility: VisibilityPublic,
				}},
				Contact: Contact{
					Email: "contact@example.com",
					Profiles: []Profile{{
						ID: "github", Platform: "GitHub", URL: "https://github.com/example", Visibility: VisibilityPublic,
					}},
				},
			},
		},
		"an empty document with only contact is valid": {
			doc: Document{Contact: Contact{Email: "contact@example.com"}},
		},
		"an empty document with no contact email fails": {
			doc:     Document{},
			wantErr: true,
		},
		"a missing required field fails": {
			doc: Document{
				Experience: []ExperienceEntry{{ID: "exp-1", Start: "2020-01", Visibility: VisibilityPublic}},
				Contact:    Contact{Email: "contact@example.com"},
			},
			wantErr: true,
		},
		"a missing id fails": {
			doc: Document{
				Experience: []ExperienceEntry{{Employer: "Acme", Title: "Engineer", Start: "2020-01", Visibility: VisibilityPublic}},
				Contact:    Contact{Email: "contact@example.com"},
			},
			wantErr: true,
		},
		"an unknown skill level fails": {
			doc: Document{
				Skills:  []SkillEntry{{ID: "skill-1", Name: "Kubernetes", Level: "guru", Visibility: VisibilityPublic}},
				Contact: Contact{Email: "contact@example.com"},
			},
			wantErr: true,
		},
		"an unknown language level fails": {
			doc: Document{
				Languages: []LanguageEntry{{ID: "lang-1", Name: "German", Level: "native-ish", Visibility: VisibilityPublic}},
				Contact:   Contact{Email: "contact@example.com"},
			},
			wantErr: true,
		},
		"an unknown visibility fails": {
			doc: Document{
				Skills:  []SkillEntry{{ID: "skill-1", Name: "Kubernetes", Level: LevelExpert, Visibility: "internal-only"}},
				Contact: Contact{Email: "contact@example.com"},
			},
			wantErr: true,
		},
		"a missing visibility fails": {
			doc: Document{
				Skills:  []SkillEntry{{ID: "skill-1", Name: "Kubernetes", Level: LevelExpert}},
				Contact: Contact{Email: "contact@example.com"},
			},
			wantErr: true,
		},
		"an address with an unknown visibility fails": {
			doc: Document{
				Contact: Contact{Email: "contact@example.com", Address: &Address{Visibility: "secret"}},
			},
			wantErr: true,
		},
		"a duplicate id within a section fails": {
			doc: Document{
				Skills: []SkillEntry{
					{ID: "skill-1", Name: "Kubernetes", Level: LevelExpert, Visibility: VisibilityPublic},
					{ID: "skill-1", Name: "Kubernetes, again", Level: LevelWorking, Visibility: VisibilityPublic},
				},
				Contact: Contact{Email: "contact@example.com"},
			},
			wantErr: true,
		},
		"a duplicate id across different sections passes": {
			doc: Document{
				Skills:   []SkillEntry{{ID: "shared", Name: "Kubernetes", Level: LevelExpert, Visibility: VisibilityPublic}},
				Projects: []ProjectEntry{{ID: "shared", GithubRepo: "acme/cv", Description: "A CV", Visibility: VisibilityPublic}},
				Contact:  Contact{Email: "contact@example.com"},
			},
		},
		"a duplicate profile id within contact fails": {
			doc: Document{
				Contact: Contact{
					Email: "contact@example.com",
					Profiles: []Profile{
						{ID: "github", Platform: "GitHub", URL: "https://github.com/a", Visibility: VisibilityPublic},
						{ID: "github", Platform: "GitHub", URL: "https://github.com/b", Visibility: VisibilityPublic},
					},
				},
			},
			wantErr: true,
		},
		"skill evidence against a project id that exists passes": {
			doc: Document{
				Skills:   []SkillEntry{{ID: "skill-1", Name: "Kubernetes", Level: LevelExpert, Visibility: VisibilityPublic, Evidence: []Evidence{{Type: EvidenceTypeProject, Ref: "proj-1"}}}},
				Projects: []ProjectEntry{{ID: "proj-1", GithubRepo: "acme/cv", Description: "A CV", Visibility: VisibilityPublic}},
				Contact:  Contact{Email: "contact@example.com"},
			},
		},
		"skill evidence against a project id that does not exist fails": {
			doc: Document{
				Skills:  []SkillEntry{{ID: "skill-1", Name: "Kubernetes", Level: LevelExpert, Visibility: VisibilityPublic, Evidence: []Evidence{{Type: EvidenceTypeProject, Ref: "no-such-project"}}}},
				Contact: Contact{Email: "contact@example.com"},
			},
			wantErr: true,
		},
		"skill evidence of an unknown type fails": {
			doc: Document{
				Skills:  []SkillEntry{{ID: "skill-1", Name: "Kubernetes", Level: LevelExpert, Visibility: VisibilityPublic, Evidence: []Evidence{{Type: "blog-post", Ref: "x"}}}},
				Contact: Contact{Email: "contact@example.com"},
			},
			wantErr: true,
		},
		"skill evidence with no ref fails": {
			doc: Document{
				Skills:  []SkillEntry{{ID: "skill-1", Name: "Kubernetes", Level: LevelExpert, Visibility: VisibilityPublic, Evidence: []Evidence{{Type: EvidenceTypeProject}}}},
				Contact: Contact{Email: "contact@example.com"},
			},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := Validate(tc.doc, "test")
			if tc.wantErr && err == nil {
				t.Fatalf("Validate() error = nil, want an error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

// TestValidateReportsEveryProblemTogether proves that two independent
// problems in one document both show up in the single returned error,
// rather than the second being hidden behind the first.
func TestValidateReportsEveryProblemTogether(t *testing.T) {
	t.Parallel()

	doc := Document{
		Skills: []SkillEntry{
			{ID: "skill-1", Level: "guru", Visibility: VisibilityPublic},
			{ID: "skill-1", Name: "Kubernetes", Level: LevelExpert, Visibility: "internal-only"},
		},
		Contact: Contact{Email: "contact@example.com"},
	}

	err := Validate(doc, "test")
	if err == nil {
		t.Fatal("Validate() error = nil, want an error")
	}

	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("Validate() error = %T, want a joined error", err)
	}
	problems := joined.Unwrap()

	// name missing on skills[0], level unknown on skills[0], visibility
	// unknown on skills[1], and the duplicate id across both: four
	// distinct problems, all present.
	if len(problems) < 4 {
		t.Fatalf("Validate() reported %d problems, want at least 4: %v", len(problems), problems)
	}
}
