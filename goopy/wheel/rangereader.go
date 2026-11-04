package wheel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
)

const (
	rangeBlockSize    = 256 * 1024      // cache granularity
	rangeTailSize     = 1024 * 1024     // fetched up front for the zip central directory
	rangeMaxReadahead = 4 * 1024 * 1024 // cap on one coalesced request
)

// errRangeUnsupported means the server did not honour an HTTP Range request.
var errRangeUnsupported = errors.New("wheel: server does not support range requests")

// httpReaderAt implements io.ReaderAt over HTTP Range requests. It caches
// fetched 256 KiB blocks. A miss that directly follows the previous fetch
// doubles the request length (up to rangeMaxReadahead), so sequential reads
// of adjacent zip entries coalesce into few requests.
type httpReaderAt struct {
	ctx    context.Context
	client *http.Client
	url    string
	size   int64

	mu       sync.Mutex
	blocks   map[int64][]byte
	lastEnd  int64 // block index after the last fetched run, -1 if none
	nextRun  int64 // blocks to fetch on the next sequential miss
	requests int
	received int64
}

// newHTTPReaderAt prefetches the tail of the file, which also verifies that
// the server supports range requests.
func newHTTPReaderAt(ctx context.Context, client *http.Client, url string, size int64) (*httpReaderAt, error) {
	if size <= 0 {
		return nil, errRangeUnsupported
	}
	r := &httpReaderAt{
		ctx:     ctx,
		client:  client,
		url:     url,
		size:    size,
		blocks:  make(map[int64][]byte),
		lastEnd: -1,
		nextRun: 1,
	}
	first := int64(0)
	if size > rangeTailSize {
		first = (size - rangeTailSize) / rangeBlockSize
	}
	last := (size - 1) / rangeBlockSize
	if err := r.fetch(first, last); err != nil {
		return nil, err
	}
	return r, nil
}

// Stats returns the number of range requests made and body bytes received.
func (r *httpReaderAt) Stats() (requests int, received int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.requests, r.received
}

// fetch downloads blocks [first, last] in one request and caches them.
func (r *httpReaderAt) fetch(first, last int64) error {
	start := first * rangeBlockSize
	end := (last+1)*rangeBlockSize - 1
	if end > r.size-1 {
		end = r.size - 1
	}
	req, err := http.NewRequestWithContext(r.ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	r.requests++

	if resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("status %d: %w", resp.StatusCode, errRangeUnsupported)
	}
	var cs, ce, ct int64
	cr := resp.Header.Get("Content-Range")
	if n, err := fmt.Sscanf(cr, "bytes %d-%d/%d", &cs, &ce, &ct); err != nil || n != 3 {
		return fmt.Errorf("bad Content-Range %q: %w", cr, errRangeUnsupported)
	}
	if cs != start || ce != end || ct != r.size {
		return fmt.Errorf("Content-Range %q does not match bytes=%d-%d/%d: %w", cr, start, end, r.size, errRangeUnsupported)
	}
	buf := make([]byte, end-start+1)
	if _, err := io.ReadFull(resp.Body, buf); err != nil {
		return fmt.Errorf("reading range body: %w", err)
	}
	r.received += int64(len(buf))
	for b := first; b <= last; b++ {
		lo := (b - first) * rangeBlockSize
		hi := min(lo+rangeBlockSize, int64(len(buf)))
		r.blocks[b] = buf[lo:hi]
	}
	r.lastEnd = last + 1
	return nil
}

// ReadAt implements io.ReaderAt.
func (r *httpReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("negative offset")
	}
	if off >= r.size {
		return 0, io.EOF
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	want := int64(len(p))
	eof := false
	if off+want > r.size {
		want = r.size - off
		eof = true
	}
	n := int64(0)
	for n < want {
		pos := off + n
		b := pos / rangeBlockSize
		blk, ok := r.blocks[b]
		if !ok {
			if err := r.fill(b, off+want-1); err != nil {
				return int(n), err
			}
			blk = r.blocks[b]
		}
		n += int64(copy(p[n:want], blk[pos-b*rangeBlockSize:]))
	}
	if eof {
		return int(n), io.EOF
	}
	return int(n), nil
}

// fill fetches the missing run starting at block b, covering at least up to
// byte lastByte, plus readahead when the miss continues the previous fetch.
func (r *httpReaderAt) fill(b, lastByte int64) error {
	needLast := lastByte / rangeBlockSize
	if b == r.lastEnd {
		if r.nextRun < rangeMaxReadahead/rangeBlockSize {
			r.nextRun *= 2
		}
	} else {
		r.nextRun = 1
	}
	last := max(b+r.nextRun-1, needLast)
	last = min(last, (r.size-1)/rangeBlockSize)
	// Stop the run before the first block that is already cached.
	for i := b + 1; i <= last; i++ {
		if _, ok := r.blocks[i]; ok {
			last = i - 1
			break
		}
	}
	return r.fetch(b, last)
}
