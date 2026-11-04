package cache

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
)

// compressThreshold is the size from which a value is stored compressed. The SQLite
// driver copies a value outside the Go heap, where GOMEMLIMIT cannot see it. For a
// 64 MB document that copy added about 125 MiB to peak memory.
const compressThreshold = 1 << 20

// compressedMarker starts every compressed row. The original size follows it, then
// the gzip stream. Rows from before compression have no marker and read as they are.
var compressedMarker = []byte("\x00pypx-gzip\x00")

// encodeRow returns the bytes to store for value. A small value that starts with the
// marker is compressed too, so a row with the marker is always a compressed row.
func encodeRow(value []byte) ([]byte, error) {
	if len(value) < compressThreshold && !bytes.HasPrefix(value, compressedMarker) {
		return value, nil
	}
	var buf bytes.Buffer
	buf.Grow(len(value)/8 + len(compressedMarker) + binary.MaxVarintLen64)
	buf.Write(compressedMarker)
	buf.Write(binary.AppendUvarint(nil, uint64(len(value))))
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(value); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if buf.Len() >= len(value) && !bytes.HasPrefix(value, compressedMarker) {
		return value, nil
	}
	return buf.Bytes(), nil
}

// decodeRow returns the value a row holds.
func decodeRow(row []byte) ([]byte, error) {
	if !bytes.HasPrefix(row, compressedMarker) {
		return row, nil
	}
	rest := row[len(compressedMarker):]
	size, n := binary.Uvarint(rest)
	if n <= 0 {
		return nil, fmt.Errorf("compressed row: bad size")
	}
	zr, err := gzip.NewReader(bytes.NewReader(rest[n:]))
	if err != nil {
		return nil, fmt.Errorf("compressed row: %w", err)
	}
	defer func() { _ = zr.Close() }()
	// Deflate cannot exceed about 1032:1, so a larger stated size is a bad row. It
	// must not cause a huge allocation.
	if size > uint64(len(row))*1100 {
		return nil, fmt.Errorf("compressed row: size %d too large for %d bytes", size, len(row))
	}
	value := make([]byte, size)
	if _, err := io.ReadFull(zr, value); err != nil {
		return nil, fmt.Errorf("compressed row: %w", err)
	}
	if extra, _ := zr.Read(make([]byte, 1)); extra != 0 {
		return nil, fmt.Errorf("compressed row: longer than size %d", size)
	}
	return value, nil
}
