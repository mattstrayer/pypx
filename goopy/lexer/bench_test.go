package lexer

import (
	"os"
	"testing"

	"github.com/pypx/goopy/token"
)

func BenchmarkLex(b *testing.B) {
	src, err := os.ReadFile("../internal/testdata/sample.py")
	if err != nil {
		b.Fatalf("reading sample.py: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		l := New(src)
		for {
			tok := l.Next()
			if tok.Type == token.EOF {
				break
			}
		}
	}
	b.SetBytes(int64(len(src)))
}

func FuzzLex(f *testing.F) {
	// Seed with representative Python snippets.
	f.Add([]byte("def hello(name: str) -> str:\n    pass\n"))
	f.Add([]byte("class Foo(Bar):\n    \"\"\"Docstring.\"\"\"\n    x: int = 0\n"))
	f.Add([]byte("import os\nfrom typing import Optional\n"))
	f.Add([]byte("x = [i for i in range(10)]\n"))
	f.Add([]byte("f\"hello {name!r:>10}\"\n"))
	f.Add([]byte("0xFF_FF + 1.0e-3j\n"))
	f.Add([]byte(""))
	f.Add([]byte("\n\n\n"))

	f.Fuzz(func(t *testing.T, src []byte) {
		l := New(src)
		for i := 0; i < 100_000; i++ {
			tok := l.Next()
			if tok.Type == token.EOF {
				return
			}
		}
		// If we hit 100k tokens without EOF, the lexer is probably stuck.
		t.Errorf("lexer produced 100k tokens without EOF on %d bytes", len(src))
	})
}
