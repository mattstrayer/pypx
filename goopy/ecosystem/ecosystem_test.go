package ecosystem

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/pypx/goopy"
)

type packageEntry struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type packageList struct {
	Packages []packageEntry `json:"packages"`
}

func TestEcosystem(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ecosystem tests in short mode")
	}

	data, err := os.ReadFile("packages.json")
	if err != nil {
		t.Fatalf("reading packages.json: %v", err)
	}

	var list packageList
	if err := json.Unmarshal(data, &list); err != nil {
		t.Fatalf("parsing packages.json: %v", err)
	}

	for _, pkg := range list.Packages {
		t.Run(pkg.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			result, err := goopy.ExtractFromPyPI(ctx, pkg.Name, pkg.Version)
			if err != nil {
				t.Fatalf("ExtractFromPyPI(%s, %s): %v", pkg.Name, pkg.Version, err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if len(result.Modules) == 0 {
				t.Error("no modules extracted")
			}

			totalFuncs := 0
			totalClasses := 0
			for _, mod := range result.Modules {
				totalFuncs += len(mod.Functions)
				totalClasses += len(mod.Classes)
			}

			t.Logf("%s: %d modules, %d functions, %d classes",
				pkg.Name, len(result.Modules), totalFuncs, totalClasses)

			if totalFuncs == 0 && totalClasses == 0 {
				t.Errorf("no functions or classes found in %s", pkg.Name)
			}
		})
	}
}
