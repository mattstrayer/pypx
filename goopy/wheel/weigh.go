package wheel

import (
	"archive/zip"
	"context"
	"fmt"
	"strings"
)

// unreadInflateRatio estimates inflated source per wheel byte when the central
// directory cannot be read. Pure-Python wheels reach about 4.2 (sympy, transformers).
const unreadInflateRatio = 5

// Weigh estimates the bytes an extraction of this package holds in memory: the
// inflated sources plus the compressed bytes they are read from. It transfers
// the package metadata and the wheel's central directory, not the sources.
//
// Wheel size alone does not predict this. pyarrow's wheel is 27.6 MiB with
// 2.7 MiB of source, and sympy's is 6 MiB with 25 MiB of source.
//
// Weigh returns an error only if the metadata cannot be read. A wheel that Fetch
// would reject weighs nothing, because Fetch fails before it reads any data.
func (s *Source) Weigh(ctx context.Context, name, version string) (int64, error) {
	wheels, err := s.fetchWheelURLs(ctx, name, version)
	if err != nil {
		return 0, fmt.Errorf("fetching wheel URLs: %w", err)
	}
	if len(wheels) == 0 {
		return 0, nil
	}
	w := selectWheel(wheels)
	if w.Size <= rangeThreshold && w.Size > s.MaxSize {
		return 0, nil
	}
	limit := s.MaxRangeSize
	if limit <= 0 {
		limit = DefaultMaxRangeSize
	}
	if w.Size > limit {
		return 0, nil
	}

	unread := unreadInflateRatio*w.Size + w.Size
	ra, err := newHTTPReaderAt(ctx, s.HTTPClient, w.URL, w.Size)
	if err != nil {
		return unread, nil
	}
	r, err := zip.NewReader(ra, w.Size)
	if err != nil {
		return unread, nil
	}

	var source, compressed int64
	for _, f := range r.File {
		if isSourceEntry(f.Name) {
			source += int64(f.UncompressedSize64)
			compressed += int64(f.CompressedSize64)
		}
	}
	if w.Size <= rangeThreshold {
		// fetchWheel downloads a small wheel whole, so all of it is held.
		return source + w.Size, nil
	}
	return source + compressed, nil
}

// isSourceEntry reports whether a zip entry counts toward an extraction's weight.
// Stub files count, since a .pyi is parsed like a .py.
func isSourceEntry(name string) bool {
	return !strings.Contains(name, "__pycache__") &&
		(strings.HasSuffix(name, ".py") || strings.HasSuffix(name, ".pyi"))
}
