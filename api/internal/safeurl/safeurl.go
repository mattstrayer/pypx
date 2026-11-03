// Package safeurl decides which URLs from third-party content may reach an
// href or src attribute. It allows known schemes and refuses everything else,
// so a new or odd scheme cannot pass by accident.
package safeurl

import "strings"

var linkSchemes = map[string]bool{
	"http":   true,
	"https":  true,
	"mailto": true,
	"ftp":    true,
	"tel":    true,
}

var rasterImagePrefixes = []string{"image/png;", "image/gif;", "image/jpeg;", "image/webp;"}

// Link reports whether u may be the href of a link. Relative URLs are allowed.
func Link(u string) bool {
	scheme, _, ok := split(u)
	return ok && (scheme == "" || linkSchemes[scheme])
}

// Image reports whether u may be the src of an image. Relative URLs and raster
// data URLs are allowed. SVG data URLs are refused.
func Image(u string) bool {
	scheme, rest, ok := split(u)
	if !ok {
		return false
	}
	switch scheme {
	case "", "http", "https":
		return true
	case "data":
		rest = strings.ToLower(rest)
		for _, prefix := range rasterImagePrefixes {
			if strings.HasPrefix(rest, prefix) {
				return true
			}
		}
	}
	return false
}

// split returns the lower-case scheme of u and the text after it. The scheme is
// empty for a relative URL. ok is false when u holds a control character or an
// empty scheme.
//
// Browsers delete tabs and newlines from a URL and trim control characters from
// its ends before they read the scheme, so "java\tscript:" runs as javascript.
// Valid URLs hold no control characters, so split refuses them all and does not
// have to copy the browser's clean-up.
func split(u string) (scheme, rest string, ok bool) {
	u = strings.TrimSpace(u)
	for i := 0; i < len(u); i++ {
		if u[i] < 0x20 || u[i] == 0x7f {
			return "", "", false
		}
	}
	end := strings.IndexAny(u, ":/?#")
	if end < 0 || u[end] != ':' {
		return "", u, true
	}
	if end == 0 {
		return "", "", false
	}
	return strings.ToLower(u[:end]), u[end+1:], true
}
