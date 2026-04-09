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
