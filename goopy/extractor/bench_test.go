package extractor

import (
	"os"
	"testing"

	"github.com/pypx/goopy/parser"
)

func BenchmarkExtractModule(b *testing.B) {
	src, err := os.ReadFile("../internal/testdata/sample.py")
	if err != nil {
		b.Fatalf("reading sample.py: %v", err)
	}

	// Pre-parse once to isolate extraction cost.
	p := parser.New(src)
	mod := p.Parse()

	e := New()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		e.ExtractModule("sample", mod)
	}
}

func BenchmarkParseAndExtract(b *testing.B) {
	src, err := os.ReadFile("../internal/testdata/sample.py")
	if err != nil {
		b.Fatalf("reading sample.py: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		p := parser.New(src)
		mod := p.Parse()
		e := New()
		e.ExtractModule("sample", mod)
	}
	b.SetBytes(int64(len(src)))
}
