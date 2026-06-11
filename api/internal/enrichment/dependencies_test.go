package enrichment

import (
	"testing"
)

func TestParseDependencies(t *testing.T) {
	input := []string{
		`charset-normalizer (<4,>=2)`,
		`idna (<4,>=2.5)`,
		`urllib3 (<3,>=1.21.1)`,
		`certifi (>=2017.4.17)`,
		`PySocks (!=1.5.7,>=1.5.6) ; extra == "socks"`,
		`chardet (<6,>=3.0.2) ; extra == "security"`,
	}

	tree := ParseDependencies(input)

	if len(tree.Required) != 4 {
		t.Fatalf("expected 4 required deps, got %d", len(tree.Required))
	}

	if tree.Required[0].Name != "charset-normalizer" {
		t.Errorf("expected first dep name 'charset-normalizer', got %q", tree.Required[0].Name)
	}
	if tree.Required[0].Constraint != "<4,>=2" {
		t.Errorf("expected first dep constraint '<4,>=2', got %q", tree.Required[0].Constraint)
	}

	if len(tree.Extras) != 2 {
		t.Fatalf("expected 2 extras groups, got %d", len(tree.Extras))
	}

	socks, ok := tree.Extras["socks"]
	if !ok {
		t.Fatal("expected 'socks' extras group")
	}
	if len(socks) != 1 {
		t.Fatalf("expected 1 dep in socks group, got %d", len(socks))
	}
	if socks[0].Name != "PySocks" {
		t.Errorf("expected socks dep name 'PySocks', got %q", socks[0].Name)
	}

	if _, ok := tree.Extras["security"]; !ok {
		t.Fatal("expected 'security' extras group")
	}
}

func TestParseDependenciesEmpty(t *testing.T) {
	tree := ParseDependencies(nil)

	if len(tree.Required) != 0 {
		t.Errorf("expected 0 required deps, got %d", len(tree.Required))
	}
	if len(tree.Extras) != 0 {
		t.Errorf("expected 0 extras groups, got %d", len(tree.Extras))
	}
	if tree.Required == nil {
		t.Error("Required slice should not be nil")
	}
	if tree.Extras == nil {
		t.Error("Extras map should not be nil")
	}
}

func TestParseSingleDep(t *testing.T) {
	tree := ParseDependencies([]string{"click (>=7.0)"})

	if len(tree.Required) != 1 {
		t.Fatalf("expected 1 required dep, got %d", len(tree.Required))
	}
	if tree.Required[0].Name != "click" {
		t.Errorf("expected dep name 'click', got %q", tree.Required[0].Name)
	}
}

// TestParseDepsInlineConstraint covers the inline PEP 508 form (no parentheses),
// e.g. "botocore<1.43.0,>=1.42.88" — these are characterization tests that pin
// current parser behavior. Cases marked SUSPECT flag behavior that may be a bug.
func TestParseDepsInlineConstraint(t *testing.T) {
	cases := []struct {
		input      string
		wantName   string
		wantConstr string
		suspect    string // non-empty if behavior looks questionable
	}{
		{
			input:      "botocore<1.43.0,>=1.42.88",
			wantName:   "botocore",
			wantConstr: "<1.43.0,>=1.42.88",
		},
		{
			input:      "requests>=2.0",
			wantName:   "requests",
			wantConstr: ">=2.0",
		},
		{
			input:      "urllib3~=2.0",
			wantName:   "urllib3",
			wantConstr: "~=2.0",
		},
		{
			input:      "packaging!=22.0",
			wantName:   "packaging",
			wantConstr: "!=22.0",
		},
		{
			input:      "flask",
			wantName:   "flask",
			wantConstr: "",
		},
		{
			// Space before operator — TrimSpace on name side should handle this.
			input:      "requests >=2.0",
			wantName:   "requests",
			wantConstr: ">=2.0",
		},
		{
			// SUSPECT: `[` is not in the operator regex so the bracket group is kept
			// in the name. Current behavior: Name="requests[socks]", Constraint=">=1.5".
			// This differs from PEP 508 where "requests[socks]" is valid name+extras
			// notation — a correct parser would separate the extras bracket. Pinned as-is.
			input:      "requests[socks]>=1.5",
			wantName:   "requests[socks]",
			wantConstr: ">=1.5",
			suspect:    "bracket extras kept in name — parseDep does not strip [extras] from inline form",
		},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			dep := parseDep(tc.input)
			if dep.Name != tc.wantName {
				t.Errorf("Name: got %q, want %q", dep.Name, tc.wantName)
			}
			if dep.Constraint != tc.wantConstr {
				t.Errorf("Constraint: got %q, want %q", dep.Constraint, tc.wantConstr)
			}
			if tc.suspect != "" {
				t.Logf("SUSPECT (characterization): %s", tc.suspect)
			}
		})
	}
}

// TestParseDependenciesInlineWithMarker verifies that an inline-constraint dep with
// a non-extras marker (python_version) is routed to Required, not Extras, and that
// the marker is stripped from the constraint before storage.
func TestParseDependenciesInlineWithMarker(t *testing.T) {
	input := []string{`idna>=2.5 ; python_version < "3.9"`}
	tree := ParseDependencies(input)

	if len(tree.Required) != 1 {
		t.Fatalf("expected 1 required dep, got %d", len(tree.Required))
	}
	dep := tree.Required[0]
	if dep.Name != "idna" {
		t.Errorf("Name: got %q, want %q", dep.Name, "idna")
	}
	if dep.Constraint != ">=2.5" {
		t.Errorf("Constraint: got %q, want %q", dep.Constraint, ">=2.5")
	}
	if len(tree.Extras) != 0 {
		t.Errorf("expected 0 extras groups, got %d", len(tree.Extras))
	}
}
