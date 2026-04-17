package handler

import (
	"strings"
)

// stubRegistry maps lowercase PyPI package names to their known type stub packages.
// Only add entries for verified, well-maintained stub packages.
var stubRegistry = map[string]string{
	"django":              "django-stubs",
	"djangorestframework": "djangorestframework-stubs",
	"requests":            "types-requests",
	"boto3":               "boto3-stubs",
	"pandas":              "pandas-stubs",
	"pillow":              "types-Pillow",
	"redis":               "types-redis",
	"pyyaml":              "types-PyYAML",
	"stripe":              "types-stripe",
}

// lookupStubPackage returns the stub package name for a source package, if known.
func lookupStubPackage(name string) (string, bool) {
	stub, ok := stubRegistry[strings.ToLower(name)]
	return stub, ok
}
