package conformance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// knownFailures lists fixtures the parser fails today, keyed by fixture
// filename with a one-line reason. A listed fixture that passes fails the test.
var knownFailures = map[string]string{
	"call_args.py":        "operators, conditional and generator expressions inside call arguments",
	"compound_headers.py": "compound statement headers with complex expressions",
	"dict_unpack.py":      "** unpacking inside dict displays",
	"loop_else.py":        "else clauses on for, while and async for",
	"match_guard.py":      "match case with star pattern and guard",
	"matmul.py":           "@ and @= operators",
	"relative_imports.py": "relative imports with four or more dots",
	"semicolons.py":       "semicolon-separated statements in a one-line block",
	"subscripts.py":       "slices and tuple subscripts",
	"tuples.py":           "unparenthesised tuples and starred expressions",
	"walrus.py":           "assignment expressions in if headers",
	"yield_expr.py":       "yield used as an expression",
}

func TestFixtures(t *testing.T) {
	files, err := filepath.Glob("testdata/*.py")
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures found: %v", err)
	}
	for _, path := range files {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(strings.TrimSuffix(path, ".py") + ".golden.json")
			if err != nil {
				t.Fatal(err)
			}
			var want []Def
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatal(err)
			}
			Sort(want)

			got, errs := Parse(src)
			var problems []string
			for _, e := range errs {
				problems = append(problems, "parse error: "+e.Error())
			}
			if !reflect.DeepEqual(got, want) {
				problems = append(problems, "defs differ from golden")
			}

			reason, known := knownFailures[name]
			if known {
				if len(problems) == 0 {
					t.Fatalf("%s passes: remove from knownFailures", name)
				}
				t.Logf("known failure (%s): %d problems, first: %s", reason, len(problems), problems[0])
				return
			}
			if len(problems) > 0 {
				for _, p := range problems {
					t.Error(p)
				}
				if !reflect.DeepEqual(got, want) {
					t.Logf("got:  %v", got)
					t.Logf("want: %v", want)
				}
			}
		})
	}
}
