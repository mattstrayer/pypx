package cache

import (
	"bytes"
	"crypto/rand"
	"testing"
	"time"
)

func rawRow(t *testing.T, c *Cache, key string) []byte {
	t.Helper()
	var row []byte
	if err := c.db.QueryRow(`SELECT value FROM cache WHERE key = ?`, key).Scan(&row); err != nil {
		t.Fatal(err)
	}
	return row
}

func newCompressCache(t *testing.T) *Cache {
	t.Helper()
	c, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	c.db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestCompress_RoundTrip(t *testing.T) {
	incompressible := make([]byte, compressThreshold+10)
	if _, err := rand.Read(incompressible); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		value      []byte
		compressed bool
	}{
		{"small", []byte(`{"a":1}`), false},
		{"empty", []byte{}, false},
		{"below threshold", bytes.Repeat([]byte("x"), compressThreshold-1), false},
		{"at threshold", bytes.Repeat([]byte("x"), compressThreshold), true},
		{"large", bytes.Repeat([]byte(`{"name":"value"},`), 1<<18), true},
		{"does not shrink", incompressible, false},
		{"small with the marker", append(bytes.Clone(compressedMarker), "plain"...), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newCompressCache(t)
			if err := c.Set("k", tc.value, 0); err != nil {
				t.Fatal(err)
			}
			if got := bytes.HasPrefix(rawRow(t, c, "k"), compressedMarker); got != tc.compressed {
				t.Errorf("row compressed = %v, want %v", got, tc.compressed)
			}
			got, fresh, err := c.Get("k", time.Hour)
			if err != nil || !fresh || !bytes.Equal(got, tc.value) {
				t.Errorf("Get = (%d bytes, %v, %v), want %d bytes", len(got), fresh, err, len(tc.value))
			}
		})
	}
}

func TestCompress_RowFromBeforeCompressionReads(t *testing.T) {
	c := newCompressCache(t)
	plain := bytes.Repeat([]byte("x"), compressThreshold*2)
	if _, err := c.db.Exec(`INSERT INTO cache (key, value, created_at) VALUES ('k', ?, ?)`, plain, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	got, _, err := c.Get("k", time.Hour)
	if err != nil || !bytes.Equal(got, plain) {
		t.Errorf("Get = (%d bytes, %v)", len(got), err)
	}
}

func TestCompress_UnreadableRowIsAMiss(t *testing.T) {
	valid, err := encodeRow(bytes.Repeat([]byte("x"), compressThreshold))
	if err != nil {
		t.Fatal(err)
	}
	huge := append(bytes.Clone(compressedMarker), 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f)
	for _, tc := range []struct {
		name string
		row  []byte
	}{
		{"marker only", compressedMarker},
		{"truncated", valid[:len(valid)/2]},
		{"not gzip", append(bytes.Clone(compressedMarker), 5, 'n', 'o', 't', 'g', 'z')},
		{"size far too large", huge},
		{"size too small", shrinkSize(t, valid)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newCompressCache(t)
			if _, err := c.db.Exec(`INSERT INTO cache (key, value, created_at) VALUES ('k', ?, ?)`, tc.row, time.Now().Unix()); err != nil {
				t.Fatal(err)
			}
			got, fresh, err := c.Get("k", time.Hour)
			if got != nil || fresh || err != nil {
				t.Errorf("Get = (%d bytes, %v, %v), want a miss", len(got), fresh, err)
			}
		})
	}
}

// shrinkSize rewrites a compressed row to state a smaller size than it holds.
func shrinkSize(t *testing.T, row []byte) []byte {
	t.Helper()
	rest := row[len(compressedMarker):]
	n := 0
	for rest[n]&0x80 != 0 {
		n++
	}
	out := append(bytes.Clone(compressedMarker), 1)
	return append(out, rest[n+1:]...)
}

func TestCompress_MemoryCachePromotesDecoded(t *testing.T) {
	c := newCompressCache(t)
	value := bytes.Repeat([]byte("doc"), compressThreshold)
	if err := c.Set("k", value, 0); err != nil {
		t.Fatal(err)
	}
	mc := NewMemoryCache(c, 10)
	for range 2 { // the first read comes from SQLite, the second from memory
		got, _, err := mc.Get("k", time.Hour)
		if err != nil || !bytes.Equal(got, value) {
			t.Fatalf("Get = (%d bytes, %v)", len(got), err)
		}
	}
}
