package handler

import (
	"context"

	"github.com/pypx/api/internal/pypi"
)

// packageFetcher is satisfied by *PackageHandler.
type packageFetcher interface {
	FetchPackage(ctx context.Context, name string) (*pypi.PyPIResponse, error)
}
