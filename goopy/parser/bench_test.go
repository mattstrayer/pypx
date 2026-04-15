package parser

import (
	"os"
	"testing"
)

func BenchmarkParse(b *testing.B) {
	src, err := os.ReadFile("../internal/testdata/sample.py")
	if err != nil {
		b.Fatalf("reading sample.py: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		p := New(src)
		p.Parse()
	}
	b.SetBytes(int64(len(src)))
}

func FuzzParse(f *testing.F) {
	f.Add([]byte("def hello(name: str) -> str:\n    \"\"\"Doc.\"\"\"\n    pass\n"))
	f.Add([]byte("class Foo(Bar, metaclass=ABCMeta):\n    x: int\n    def method(self) -> None: pass\n"))
	f.Add([]byte("from typing import Optional, Union\nimport os\n"))
	f.Add([]byte("type Alias = int | str\n"))
	f.Add([]byte("x = [i for i in range(10)]\ny = {k: v for k, v in d.items()}\n"))
	f.Add([]byte("a, b = 1, 2\nc = d = e = 3\n"))
	f.Add([]byte("async def fetch(url: str) -> bytes: pass\n"))
	f.Add([]byte(""))

	f.Fuzz(func(t *testing.T, src []byte) {
		p := New(src)
		// Must not panic.
		p.Parse()
	})
}
