// Package textfmt renders pypx API responses as plain-text suitable for
// agent consumption (curl + grep). Formatters are pure functions; callers
// supply already-fetched data structures from the existing handlers.
package textfmt

import (
	"fmt"
	"strings"
)

// HumanBytes formats a byte count as a short human-readable string.
// Examples: 512 → "512 B"; 1536 → "1.5 KB"; 1048576 → "1.0 MB".
func HumanBytes(n int64) string {
	const k = 1024
	if n < k {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	v := float64(n) / k
	for _, u := range units {
		if v < k {
			return fmt.Sprintf("%.1f %s", v, u)
		}
		v /= k
	}
	return fmt.Sprintf("%.1f PB", v)
}

// WriteKV writes a "key: value\n" line to b. Empty values are skipped.
func WriteKV(b *strings.Builder, key, value string) {
	if value == "" {
		return
	}
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(value)
	b.WriteByte('\n')
}
