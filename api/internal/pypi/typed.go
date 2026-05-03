package pypi

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const (
	maxWheelBytes  = 50 * 1024 * 1024 // 50 MB — skip if larger
	tailWindowSize = 64 * 1024         // 64 KB range request
)

// partialReader implements io.ReaderAt for the tail of a remote file.
// It serves reads from a buffered tail; reads outside the buffer fail.
type partialReader struct {
	data     []byte
	fileSize int64
}

func (r *partialReader) ReadAt(p []byte, off int64) (int, error) {
	bufStart := r.fileSize - int64(len(r.data))
	if off < bufStart {
		return 0, fmt.Errorf("read at offset %d is before buffered region starting at %d", off, bufStart)
	}
	localOff := off - bufStart
	if localOff >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n := copy(p, r.data[localOff:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// ExtractWheelURL returns the URL of the first bdist_wheel file in urls,
// preferring pure-python wheels (none-any). Returns "" if none found.
func ExtractWheelURL(files []ReleaseFile) string {
	var first string
	for _, f := range files {
		if f.PackageType != "bdist_wheel" {
			continue
		}
		if first == "" {
			first = f.URL
		}
		if strings.Contains(f.Filename, "none-any") {
			return f.URL
		}
	}
	return first
}

// CheckPyTyped checks whether the given wheel URL contains a py.typed marker.
// It uses an HTTP Range request to fetch only the last 64 KB (zip central directory).
// Returns false on any error or if the wheel exceeds 50 MB.
func CheckPyTyped(ctx context.Context, c *Client, wheelURL string) bool {
	// HEAD request to get file size.
	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, wheelURL, nil)
	if err != nil {
		return false
	}
	head, err := c.httpClient.Do(headReq)
	if err != nil {
		return false
	}
	head.Body.Close()

	contentLength, err := strconv.ParseInt(head.Header.Get("Content-Length"), 10, 64)
	if err != nil || contentLength <= 0 {
		return false
	}
	if contentLength > maxWheelBytes {
		return false
	}

	// Range request for the last tailWindowSize bytes (or whole file if smaller).
	start := contentLength - tailWindowSize
	if start < 0 {
		start = 0
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wheelURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", start))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return false
	}

	tail, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	// Use archive/zip with partialReader to parse the central directory.
	zr, err := zip.NewReader(&partialReader{data: tail, fileSize: contentLength}, contentLength)
	if err != nil {
		return false
	}

	for _, f := range zr.File {
		name := f.Name
		// Root-level py.typed (flat package layout).
		if name == "py.typed" {
			return true
		}
		// PEP 561: py.typed lives inside the importable package directory,
		// e.g. "requests/py.typed". Exclude .dist-info and .data directories.
		if strings.HasSuffix(name, "/py.typed") {
			parent := name[:strings.LastIndex(name, "/")]
			if !strings.HasSuffix(parent, ".dist-info") && !strings.HasSuffix(parent, ".data") {
				return true
			}
		}
	}
	return false
}
