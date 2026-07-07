package goopy

import (
	"context"
	"fmt"
	"testing"
)

func TestExtractModule(t *testing.T) {
	src := []byte("\"\"\"My module.\"\"\"\n\ndef hello(name: str) -> str:\n    \"\"\"Say hello.\n\n    Args:\n        name: The name to greet.\n\n    Returns:\n        str: A greeting string.\n    \"\"\"\n    return f\"Hello, {name}\"\n\nclass Greeter:\n    \"\"\"A greeter.\"\"\"\n\n    def __init__(self, prefix: str = \"Hello\"):\n        self.prefix = prefix\n\n    def greet(self, name: str) -> str:\n        \"\"\"Greet someone.\"\"\"\n        return f\"{self.prefix}, {name}\"\n\nclass BadInputError(ValueError):\n    \"\"\"Raised on bad input.\"\"\"\n    pass\n")

	mod, _ := ExtractModule("mymod", src)
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
	mod, errs := ExtractModule("test", src)
	if mod == nil {
		t.Fatal("ExtractModule returned nil")
	}
	if len(errs) == 0 {
		t.Error("expected parse errors for malformed source")
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
	pkg := ExtractPackage(context.Background(), "test", files, []string{"mypkg"})
	if len(pkg.Modules) != 2 {
		t.Fatalf("Modules len = %d, want 2", len(pkg.Modules))
	}
}

func TestExtractPackage_SingleFileModule(t *testing.T) {
	files := map[string][]byte{
		"six.py": []byte("def add_metaclass(cls): pass\n"),
	}
	pkg := ExtractPackage(context.Background(), "six", files, []string{"six"})
	if len(pkg.Modules) != 1 {
		t.Fatalf("Modules len = %d, want 1", len(pkg.Modules))
	}
	if pkg.Modules[0].Name != "six" {
		t.Errorf("Modules[0].Name = %q, want %q", pkg.Modules[0].Name, "six")
	}
	if len(pkg.Modules[0].Functions) != 1 {
		t.Fatalf("Functions len = %d, want 1", len(pkg.Modules[0].Functions))
	}
	if pkg.Modules[0].Functions[0].Name != "add_metaclass" {
		t.Errorf("Functions[0].Name = %q, want %q", pkg.Modules[0].Functions[0].Name, "add_metaclass")
	}
}

func TestExtractPackage_SingleFileModule_DoubleCountGuard(t *testing.T) {
	// When both a directory package and a bare top-level file of the same
	// name exist, only the directory package should be extracted.
	files := map[string][]byte{
		"six/__init__.py": []byte("def add_metaclass(cls): pass\n"),
		"six.py":          []byte("def other_func(): pass\n"),
	}
	pkg := ExtractPackage(context.Background(), "six", files, []string{"six"})
	if len(pkg.Modules) != 1 {
		t.Fatalf("Modules len = %d, want 1", len(pkg.Modules))
	}
	if pkg.Modules[0].Name != "six" {
		t.Errorf("Modules[0].Name = %q, want %q", pkg.Modules[0].Name, "six")
	}
	if len(pkg.Modules[0].Functions) != 1 || pkg.Modules[0].Functions[0].Name != "add_metaclass" {
		t.Errorf("expected the directory package's add_metaclass, got %+v", pkg.Modules[0].Functions)
	}
}

func TestExtractPackage_DocstringOnlyModule(t *testing.T) {
	// A module with only a docstring (e.g., __init__.py) should not be filtered out.
	files := map[string][]byte{
		"mypkg/__init__.py": []byte(`"""My package docstring."""
`),
	}
	pkg := ExtractPackage(context.Background(), "mypkg", files, []string{"mypkg"})
	if len(pkg.Modules) != 1 {
		t.Fatalf("Modules len = %d, want 1 (docstring-only module should be included)", len(pkg.Modules))
	}
	if pkg.Modules[0].Docstring == nil {
		t.Error("expected module docstring to be set")
	}
}

func TestExtractPackage_CancelledContext(t *testing.T) {
	// Create enough files to trigger the goroutine pool path (>4 files)
	files := make(map[string][]byte)
	for i := 0; i < 10; i++ {
		path := fmt.Sprintf("mypkg/mod%d.py", i)
		files[path] = []byte(fmt.Sprintf(`
def func_%d():
    """Function %d"""
    pass
`, i, i))
	}

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should return without panic or hang
	pkg := ExtractPackage(ctx, "mypkg", files, []string{"mypkg"})

	// Package should be non-nil (always returns a result)
	if pkg == nil {
		t.Fatal("expected non-nil package")
	}

	// With immediate cancellation, most/all modules should be skipped
	// (exact count depends on goroutine scheduling, so just verify it doesn't hang or panic)
	t.Logf("extracted %d modules with cancelled context", len(pkg.Modules))
}

func TestExtractPackage_SmallPackageCancelledContext(t *testing.T) {
	// <=4 files takes the non-goroutine path
	files := map[string][]byte{
		"mypkg/__init__.py": []byte(`"""Package docstring"""`),
		"mypkg/a.py":        []byte(`def a(): pass`),
		"mypkg/b.py":        []byte(`def b(): pass`),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pkg := ExtractPackage(ctx, "mypkg", files, []string{"mypkg"})
	if pkg == nil {
		t.Fatal("expected non-nil package")
	}
	t.Logf("extracted %d modules with cancelled context (small package)", len(pkg.Modules))
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
