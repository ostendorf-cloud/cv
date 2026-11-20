package cv

import "testing"

func TestDrift(t *testing.T) {
	t.Parallel()

	skill := func(id string) SkillEntry {
		return SkillEntry{ID: id, Name: id, Level: LevelExpert}
	}
	profile := func(id string) Profile {
		return Profile{ID: id, Platform: id, URL: "https://example.com/" + id}
	}

	tests := map[string]struct {
		de, en  Document
		wantErr bool
	}{
		"two empty documents have no drift": {},
		"matching ids in every section have no drift": {
			de: Document{
				Skills:  []SkillEntry{skill("kubernetes")},
				Contact: Contact{Profiles: []Profile{profile("github")}},
			},
			en: Document{
				Skills:  []SkillEntry{skill("kubernetes")},
				Contact: Contact{Profiles: []Profile{profile("github")}},
			},
		},
		"differing wording on a matching id has no drift": {
			de: Document{Skills: []SkillEntry{{ID: "kubernetes", Name: "Kubernetes", Level: LevelExpert}}},
			en: Document{Skills: []SkillEntry{{ID: "kubernetes", Name: "Kubernetes orchestration", Level: LevelExpert}}},
		},
		"an id only in the German file is drift": {
			de:      Document{Skills: []SkillEntry{skill("kubernetes"), skill("terraform")}},
			en:      Document{Skills: []SkillEntry{skill("kubernetes")}},
			wantErr: true,
		},
		"an id only in the English file is drift": {
			de:      Document{Skills: []SkillEntry{skill("kubernetes")}},
			en:      Document{Skills: []SkillEntry{skill("kubernetes"), skill("terraform")}},
			wantErr: true,
		},
		"a category id only in the German file is drift": {
			de:      Document{SkillCategories: []SkillCategoryEntry{{ID: "cloud", Name: "Cloud"}, {ID: "tools", Name: "Werkzeuge"}}},
			en:      Document{SkillCategories: []SkillCategoryEntry{{ID: "cloud", Name: "Cloud"}}},
			wantErr: true,
		},
		"a category id only in the English file is drift": {
			de:      Document{SkillCategories: []SkillCategoryEntry{{ID: "cloud", Name: "Cloud"}}},
			en:      Document{SkillCategories: []SkillCategoryEntry{{ID: "cloud", Name: "Cloud"}, {ID: "tools", Name: "Tools"}}},
			wantErr: true,
		},
		"matching category ids with differing names have no drift": {
			de: Document{SkillCategories: []SkillCategoryEntry{{ID: "cloud", Name: "Wolke"}}},
			en: Document{SkillCategories: []SkillCategoryEntry{{ID: "cloud", Name: "Cloud"}}},
		},
		"a contact profile only on one side is drift": {
			de:      Document{Contact: Contact{Profiles: []Profile{profile("github")}}},
			en:      Document{},
			wantErr: true,
		},
		"an id present in one section but not another section does not confuse sections": {
			de: Document{Skills: []SkillEntry{skill("shared")}},
			en: Document{Experience: []ExperienceEntry{{ID: "shared", Employer: "Acme", Title: "Engineer", Start: "2020-01"}}},
			// "shared" is missing from skills on the English side and
			// missing from experience on the German side: drift in both
			// sections, independently.
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := Drift(tc.de, tc.en)
			if tc.wantErr && err == nil {
				t.Fatalf("Drift() error = nil, want an error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Drift() error = %v, want nil", err)
			}
		})
	}
}

// TestDriftNamesEveryMissingID proves that Drift does not stop at the first
// mismatch: every id missing on either side is named in the returned error.
func TestDriftNamesEveryMissingID(t *testing.T) {
	t.Parallel()

	de := Document{Skills: []SkillEntry{
		{ID: "kubernetes", Name: "Kubernetes", Level: LevelExpert},
		{ID: "terraform", Name: "Terraform", Level: LevelWorking},
	}}
	en := Document{Skills: []SkillEntry{
		{ID: "kubernetes", Name: "Kubernetes", Level: LevelExpert},
		{ID: "ansible", Name: "Ansible", Level: LevelFamiliar},
	}}

	err := Drift(de, en)
	if err == nil {
		t.Fatal("Drift() error = nil, want an error")
	}

	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("Drift() error = %T, want a joined error", err)
	}
	// terraform (German only) and ansible (English only): two distinct
	// problems, both present.
	if got := len(joined.Unwrap()); got != 2 {
		t.Fatalf("Drift() reported %d problems, want 2: %v", got, err)
	}
}
