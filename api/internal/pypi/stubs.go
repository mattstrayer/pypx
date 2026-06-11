package pypi

import (
	"context"
	"fmt"
	"net/http"
)

// TypeSupport describes whether a package ships type information.
type TypeSupport struct {
	// Status is one of "typed", "stubs", or "untyped".
	// "typed"  — the package ships a py.typed marker (not yet checked here).
	// "stubs"  — a separate types-{name} or {name}-stubs package exists.
	// "untyped" — no type information found.
	Status       string `json:"status"`
	StubsPackage string `json:"stubs_package,omitempty"`
}

// CheckTypeSupport checks PyPI for stub packages for the given package name.
// It checks for types-{name} first, then {name}-stubs.
// Returns TypeSupport with Status "stubs" if found, "untyped" otherwise.
func CheckTypeSupport(ctx context.Context, c *Client, name string) TypeSupport {
	candidates := []string{
		"types-" + name,
		name + "-stubs",
	}
	for _, candidate := range candidates {
		if packageExists(ctx, c, candidate) {
			return TypeSupport{Status: "stubs", StubsPackage: candidate}
		}
	}
	return TypeSupport{Status: "untyped"}
}

// packageExists returns true if the named package exists on PyPI.
func packageExists(ctx context.Context, c *Client, name string) bool {
	url := fmt.Sprintf("%s/pypi/%s/json", c.baseURL, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
