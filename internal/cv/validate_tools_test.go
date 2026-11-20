package cv

import (
	"strings"
	"testing"
)

func toolsDocument(tools ...ToolEntry) Document {
	return Document{Tools: tools, Contact: Contact{Email: "contact@example.com"}}
}

func TestValidateTools(t *testing.T) {
	t.Parallel()

	ok := ToolEntry{ID: "editor", Name: "Neovim", Group: GroupEditorShell, Description: "Editor for everything."}

	tests := map[string]struct {
		doc     Document
		wantErr string
	}{
		"no tools passes": {doc: toolsDocument()},
		"a complete tool": {doc: toolsDocument(ok)},
		"every group is valid": {doc: toolsDocument(
			ToolEntry{ID: "a", Name: "A", Group: GroupWorkstation, Description: "d"},
			ToolEntry{ID: "b", Name: "B", Group: GroupEditorShell, Description: "d"},
			ToolEntry{ID: "c", Name: "C", Group: GroupCommandLine, Description: "d"},
			ToolEntry{ID: "d", Name: "D", Group: GroupHomelabHardware, Description: "d"},
			ToolEntry{ID: "e", Name: "E", Group: GroupHomelabPlatform, Description: "d"},
		)},
		"a missing id":          {doc: toolsDocument(ToolEntry{Name: "N", Group: GroupWorkstation, Description: "d"}), wantErr: "id: is required"},
		"a missing name":        {doc: toolsDocument(ToolEntry{ID: "x", Group: GroupWorkstation, Description: "d"}), wantErr: "name: is required"},
		"a missing description": {doc: toolsDocument(ToolEntry{ID: "x", Name: "N", Group: GroupWorkstation}), wantErr: "description: is required"},
		"a missing group":       {doc: toolsDocument(ToolEntry{ID: "x", Name: "N", Description: "d"}), wantErr: "group: is required"},
		"an unknown group":      {doc: toolsDocument(ToolEntry{ID: "x", Name: "N", Group: "cloud", Description: "d"}), wantErr: `group: unknown value "cloud"`},
		"a duplicate id": {doc: toolsDocument(
			ToolEntry{ID: "x", Name: "A", Group: GroupWorkstation, Description: "d"},
			ToolEntry{ID: "x", Name: "B", Group: GroupWorkstation, Description: "d"},
		), wantErr: `duplicate id "x"`},
		"an address in the description": {
			doc:     toolsDocument(ToolEntry{ID: "x", Name: "Router", Group: GroupHomelabHardware, Description: "Gateway at 192.168.0.1"}),
			wantErr: "description: an IPv4 address is not allowed",
		},
		"a hostname in the name": {
			doc:     toolsDocument(ToolEntry{ID: "x", Name: "nas.example.com", Group: GroupHomelabHardware, Description: "Storage"}),
			wantErr: "name: a hostname is not allowed",
		},
		"a port in the description": {
			doc:     toolsDocument(ToolEntry{ID: "x", Name: "Gitea", Group: GroupHomelabPlatform, Description: "Git over port 22"}),
			wantErr: "description: a port is not allowed",
		},
		"a product name passes": {
			doc: toolsDocument(ToolEntry{ID: "x", Name: "Home Assistant", Group: GroupHomelabPlatform, Description: "Runs on Node.js tooling, v1.2."}),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := Validate(tc.doc, "test")
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("Validate() error = %v, want nil", err)
			case tc.wantErr != "" && err == nil:
				t.Fatalf("Validate() error = nil, want %q", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Fatalf("Validate() error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestDriftTools(t *testing.T) {
	t.Parallel()

	tool := func(id string) ToolEntry { return ToolEntry{ID: id} }

	if err := Drift(Document{Tools: []ToolEntry{tool("a")}}, Document{Tools: []ToolEntry{tool("a")}}); err != nil {
		t.Errorf("Drift() of matching tools = %v, want nil", err)
	}
	if err := Drift(Document{Tools: []ToolEntry{tool("a")}}, Document{}); err == nil {
		t.Error("Drift() of a tool only in the German file = nil, want an error")
	}
	if err := Drift(Document{}, Document{Tools: []ToolEntry{tool("a")}}); err == nil {
		t.Error("Drift() of a tool only in the English file = nil, want an error")
	}
}
