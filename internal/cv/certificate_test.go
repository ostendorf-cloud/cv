package cv

import (
	"strings"
	"testing"
)

func TestValidateCertificateExpiry(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		issued, validUntil string
		want               string // empty means valid
	}{
		"no expiry is valid":                {issued: "2023-01"},
		"a blank expiry is valid":           {issued: "2023-01", validUntil: " \n"},
		"an expiry after issue is valid":    {issued: "2023-01", validUntil: "2026-01"},
		"an expiry in the issue month":      {issued: "2023-01", validUntil: "2023-01"},
		"an expiry before issue fails":      {issued: "2023-01", validUntil: "2022-12", want: "valid_until"},
		"an expiry in the wrong format":     {issued: "2023-01", validUntil: "2026", want: "is not a YYYY-MM date"},
		"a placeholder dash fails":          {issued: "2023-01", validUntil: "-", want: "is not a YYYY-MM date"},
		"a missing issue date fails":        {validUntil: "2026-01", want: "issued: is required"},
		"an issue date in a wrong format":   {issued: "Jan 2023", want: "issued: \"Jan 2023\" is not a YYYY-MM date"},
		"an unordered pair with bad format": {issued: "2023", validUntil: "2022-01", want: "is not a YYYY-MM date"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			doc := validDocument()
			doc.Certificates[0].Issued = tc.issued
			doc.Certificates[0].ValidUntil = tc.validUntil
			err := Validate(doc, "test")
			if tc.want == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestRenderJSONCertificateExpiry(t *testing.T) {
	t.Parallel()

	doc := Document{
		Certificates: []CertificateEntry{
			{ID: "a", Name: "N", Issuer: "I", Issued: "2023-01", ValidUntil: "2026-01"},
			{ID: "b", Name: "N", Issuer: "I", Issued: "2023-01"},
		},
		Contact: Contact{Email: "contact@example.com"},
	}
	data, err := RenderJSON(doc, "en")
	if err != nil {
		t.Fatalf("RenderJSON() error = %v", err)
	}
	for _, want := range []string{`"valid_until": "2026-01"`, `"valid_until": null`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("output lacks %s:\n%s", want, data)
		}
	}
}
