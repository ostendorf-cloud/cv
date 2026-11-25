package cv

import (
	"strings"
	"testing"
)

// TestValidateGithubRepo proves a github_repo that is not owner/name fails
// validation, so bad data fails `cv validate` and not only the enrichment.
func TestValidateGithubRepo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		repo    string
		wantErr bool
	}{
		{name: "owner and name", repo: "ostendorf-cloud/cv"},
		{name: "a dot in the name", repo: "ostendorf-cloud/ostendorf.dev"},
		{name: "no owner", repo: "cv", wantErr: true},
		{name: "a third segment", repo: "a/b/c", wantErr: true},
		{name: "a full URL", repo: "https://github.com/a/b", wantErr: true},
		{name: "a space", repo: "a/b c", wantErr: true},
		{name: "a query", repo: "a/b?x=1", wantErr: true},
		{name: "a name of one dot", repo: "a/.", wantErr: true},
		{name: "a name of two dots", repo: "a/..", wantErr: true},
		{name: "an owner of two dots", repo: "../b", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := validDocument()
			doc.Projects[0].GithubRepo = tc.repo
			err := Validate(doc, "test")
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr && !strings.Contains(err.Error(), "github_repo") {
				t.Errorf("error = %v, want it to name github_repo", err)
			}
		})
	}
}
