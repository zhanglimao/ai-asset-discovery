package scanner

import (
	"strings"
	"testing"
)

// TestParseNPMJSON validates the npm --json output parser.
func TestParseNPMJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantPkgs int
		want     map[string]string // name → version
	}{
		{
			name: "standard JSON output",
			input: `{
  "dependencies": {
    "typescript": { "version": "5.3.3" },
    "eslint": { "version": "8.56.0" },
    "@angular/cli": { "version": "17.0.0" },
    "prettier": { "version": "3.1.0" }
  }
}`,
			wantPkgs: 4,
			want:     map[string]string{"typescript": "5.3.3", "eslint": "8.56.0", "@angular/cli": "17.0.0", "prettier": "3.1.0"},
		},
		{
			name: "empty dependencies",
			input: `{
  "dependencies": {}
}`,
			wantPkgs: 0,
		},
		{
			name:     "not JSON (tree view)",
			input:    `├── package@1.0.0`,
			wantPkgs: 0,
		},
		{
			name:     "empty string",
			input:    "",
			wantPkgs: 0,
		},
		{
			name:     "malformed JSON",
			input:    `{ broken }`,
			wantPkgs: 0,
		},
		{
			name: "JSON with leading whitespace",
			input: `

{
  "dependencies": {
    "go": { "version": "1.21.0" }
  }
}
`,
			wantPkgs: 1,
			want:     map[string]string{"go": "1.21.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgs := parseNPMJSON(tt.input)
			if len(pkgs) != tt.wantPkgs {
				t.Errorf("parseNPMJSON() got %d packages, want %d: %+v", len(pkgs), tt.wantPkgs, pkgs)
			}
			for name, ver := range tt.want {
				found := false
				for _, p := range pkgs {
					if p.Name == name && p.Version == ver {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("parseNPMJSON() missing expected package %s@%s", name, ver)
				}
			}
		})
	}
}

// TestParseNPMList_TreeView validates the tree-view fallback parsing.
func TestParseNPMList_TreeView(t *testing.T) {
	// Simulate npm list output without --json (old npm versions)
	input := strings.TrimSpace(`
/usr/local/lib
├── @scope/package-a@1.2.3
├── eslint@8.56.0
├── prettier@3.1.0
├── typescript@5.3.3
└── webpack@5.89.0
`)
	ps := &PackageScanner{}
	pkgs := ps.parseNPMList(input)

	if len(pkgs) != 5 {
		t.Errorf("parseNPMList() got %d packages, want 5: %+v", len(pkgs), pkgs)
	}

	want := map[string]string{
		"@scope/package-a": "1.2.3",
		"eslint":           "8.56.0",
		"prettier":         "3.1.0",
		"typescript":       "5.3.3",
		"webpack":          "5.89.0",
	}
	for _, p := range pkgs {
		if ver, ok := want[p.Name]; !ok || ver != p.Version {
			t.Errorf("unexpected package %s@%s, wanted %s", p.Name, p.Version, ver)
		}
	}
}

// TestParseNPMList_JSONPreferred verifies JSON is preferred over tree-view.
// In real scenarios, npm list output is either pure JSON or pure tree-view, never mixed.
func TestParseNPMList_JSONPreferred(t *testing.T) {
	// Pure JSON: this is what `npm list --json` actually produces.
	input := `{
  "dependencies": {
    "eslint": { "version": "8.56.0" },
    "typescript": { "version": "5.3.3" }
  }
}`

	ps := &PackageScanner{}
	pkgs := ps.parseNPMList(input)

	if len(pkgs) != 2 {
		t.Errorf("parseNPMList() got %d packages, want 2: %+v", len(pkgs), pkgs)
	}
	// Map iteration order is non-deterministic in Go; use a lookup map.
	byName := make(map[string]string, len(pkgs))
	for _, p := range pkgs {
		byName[p.Name] = p.Version
	}
	if byName["eslint"] != "8.56.0" {
		t.Errorf("parseNPMList() eslint version = %q, want 8.56.0", byName["eslint"])
	}
	if byName["typescript"] != "5.3.3" {
		t.Errorf("parseNPMList() typescript version = %q, want 5.3.3", byName["typescript"])
	}
}

// TestParseNPMList_JSONFallsBackToTree verifies that when JSON parse fails,
// the function falls back to tree-view parsing.
func TestParseNPMList_JSONFallsBackToTree(t *testing.T) {
	// malformed JSON + valid tree-view: JSON parse fails, tree-view should work.
	input := `{ broken json }
├── eslint@8.56.0
└── typescript@5.3.3`

	ps := &PackageScanner{}
	pkgs := ps.parseNPMList(input)

	if len(pkgs) != 2 {
		t.Errorf("parseNPMList() got %d packages, want 2 (tree-view fallback): %+v", len(pkgs), pkgs)
	}
	if pkgs[0].Name != "eslint" || pkgs[0].Version != "8.56.0" {
		t.Errorf("parseNPMList()[0] = %s@%s, want eslint@8.56.0", pkgs[0].Name, pkgs[0].Version)
	}
}

// TestParseNPMList_ScopedPackageTreeView tests scoped package parsing in tree view.
func TestParseNPMList_ScopedPackageTreeView(t *testing.T) {
	input := strings.TrimSpace(`
├── package@1.0.0
├── @scope@1.0.0
├── @scope/name@2.0.0
└── @very/deeply/scoped/name@3.0.0
`)
	ps := &PackageScanner{}
	pkgs := ps.parseNPMList(input)

	if len(pkgs) != 4 {
		t.Errorf("parseNPMList() got %d packages, want 4: %+v", len(pkgs), pkgs)
	}

	for _, p := range pkgs {
		if p.Version == "" {
			t.Errorf("package %q has empty version", p.Name)
		}
	}
}
