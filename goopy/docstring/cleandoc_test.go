package docstring

import "testing"

// Expected values come from Python's inspect.cleandoc.
func TestCleandoc(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Summary.\n\n    Body\n      indented\n    ", "Summary.\n\nBody\n  indented"},
		{"  lead\n  second", "lead\nsecond"},
		{"\tTab\n\t\tDeep\n\tBack", "Tab\n        Deep\nBack"},
		{"Only one line   ", "Only one line   "},
		{"\n\n   a\n  b\n\n\n", " a\nb"},
		{"x\n\n   y\n\n\n   z\n   \n", "x\n\ny\n\n\nz"},
	}
	for _, tt := range tests {
		if got := cleandoc(tt.in); got != tt.want {
			t.Errorf("cleandoc(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
