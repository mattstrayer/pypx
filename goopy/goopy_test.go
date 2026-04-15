package goopy

import (
	"context"
	"testing"
)

func TestExtractModule(t *testing.T) {
	src := []byte("\"\"\"My module.\"\"\"\n\ndef hello(name: str) -> str:\n    \"\"\"Say hello.\n\n    Args:\n        name: The name to greet.\n\n    Returns:\n        str: A greeting string.\n    \"\"\"\n    return f\"Hello, {name}\"\n\nclass Greeter:\n    \"\"\"A greeter.\"\"\"\n\n    def __init__(self, prefix: str = \"Hello\"):\n        self.prefix = prefix\n\n    def greet(self, name: str) -> str:\n        \"\"\"Greet someone.\"\"\"\n        return f\"{self.prefix}, {name}\"\n\nclass BadInputError(ValueError):\n    \"\"\"Raised on bad input.\"\"\"\n    pass\n")

	mod := ExtractModule("mymod", src)
	if mod.Name != "mymod" {
		t.Errorf("Name = %q", mod.Name)
	}
	if len(mod.Functions) != 1 {
		t.Fatalf("Functions len = %d, want 1", len(mod.Functions))
	}
	if mod.Functions[0].Name != "hello" {
		t.Errorf("Functions[0].Name = %q", mod.Functions[0].Name)
	}
	if len(mod.Classes) < 1 {
		t.Fatal("expected at least 1 class")
	}
	found := false
	for _, cls := range mod.Classes {
		if cls.Name == "Greeter" {
			found = true
		}
	}
	if !found {
		t.Error("Greeter class not found")
	}
}

func TestExtractModuleErrors(t *testing.T) {
	// Malformed source should not panic, should return partial results.
	src := []byte("def broken(:\n    pass\ndef good():\n    pass\n")
	mod := ExtractModule("test", src)
	if mod == nil {
		t.Fatal("ExtractModule returned nil")
	}
	found := false
	for _, fn := range mod.Functions {
		if fn.Name == "good" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'good' function in partial results")
	}
}

func TestPathToModuleName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"pkg/mod.py", "pkg.mod"},
		{"pkg/__init__.py", "pkg"},
		{"pkg/sub/deep.py", "pkg.sub.deep"},
	}
	for _, tt := range tests {
		got := pathToModuleName(tt.path)
		if got != tt.want {
			t.Errorf("pathToModuleName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestExtractPackage(t *testing.T) {
	files := map[string][]byte{
		"mypkg/__init__.py": []byte("\"\"\"My package.\"\"\"\ndef top_func(): pass\n"),
		"mypkg/sub.py":      []byte("def sub_func(): pass\n"),
		"other/foo.py":      []byte("def other_func(): pass\n"),
	}
	pkg := ExtractPackage("test", files, []string{"mypkg"})
	if len(pkg.Modules) != 2 {
		t.Fatalf("Modules len = %d, want 2", len(pkg.Modules))
	}
}

func TestExtractFromPyPI_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	pkg, err := ExtractFromPyPI(ctx, "click", "8.1.8")
	if err != nil {
		t.Fatalf("ExtractFromPyPI: %v", err)
	}
	if pkg.Name != "click" {
		t.Errorf("Name = %q, want %q", pkg.Name, "click")
	}
	if len(pkg.Modules) == 0 {
		t.Error("no modules extracted")
	}

	totalFuncs := 0
	for _, mod := range pkg.Modules {
		totalFuncs += len(mod.Functions)
	}
	if totalFuncs == 0 {
		t.Error("no functions found in click")
	}
}
