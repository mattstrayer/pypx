package wheel

import "testing"

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"requests", "requests"},
		{"My-Package", "my_package"},
		{"my.package", "my_package"},
		{"My-Cool.Package", "my_cool_package"},
		{"UPPER", "upper"},
	}
	for _, tt := range tests {
		got := NormalizeName(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseTopLevelTxt(t *testing.T) {
	tests := []struct {
		content string
		want    int
	}{
		{"requests\n", 1},
		{"pkg1\npkg2\n", 2},
		{"  spaced  \n", 1},
		{"", 0},
		{"\n\n", 0},
	}
	for _, tt := range tests {
		got := parseTopLevelTxt(tt.content)
		if len(got) != tt.want {
			t.Errorf("parseTopLevelTxt(%q) = %v (len %d), want len %d", tt.content, got, len(got), tt.want)
		}
	}
}

func TestSelectWheel(t *testing.T) {
	wheels := []WheelFile{
		{Filename: "pkg-1.0-cp39-linux.whl", URL: "https://example.com/linux.whl"},
		{Filename: "pkg-1.0-py3-none-any.whl", URL: "https://example.com/any.whl"},
	}
	got := selectWheel(wheels)
	if got != "https://example.com/any.whl" {
		t.Errorf("selectWheel() = %q, want any.whl URL", got)
	}
}

func TestSelectWheelFallback(t *testing.T) {
	wheels := []WheelFile{
		{Filename: "pkg-1.0-cp39-linux.whl", URL: "https://example.com/linux.whl"},
	}
	got := selectWheel(wheels)
	if got != "https://example.com/linux.whl" {
		t.Errorf("selectWheel() = %q, want fallback URL", got)
	}
}

func TestInferTopLevel(t *testing.T) {
	files := map[string][]byte{
		"mypkg/__init__.py":             {},
		"mypkg/mod.py":                  {},
		"mypkg-1.0.dist-info/METADATA":  {},
	}
	got := inferTopLevel(files, "mypkg")
	if len(got) != 1 || got[0] != "mypkg" {
		t.Errorf("inferTopLevel() = %v, want [mypkg]", got)
	}
}
