package middleware

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// txtTwins lists the JSON routes that have a plain-text ".txt" twin.
// {name} segments never contain "/" (PEP 503 normalized names); {symbol}
// may contain dots but never a slash.
var txtTwins = []*regexp.Regexp{
	regexp.MustCompile(`^/api/packages/[^/]+$`),
	regexp.MustCompile(`^/api/packages/[^/]+/(changelog|security|extras|stats|docs|diff)$`),
	regexp.MustCompile(`^/api/packages/[^/]+/docs/[^/]+$`),
	regexp.MustCompile(`^/api/(search|compare|popular)$`),
}

// NegotiateText rewrites the request path to its ".txt" twin when the
// client's Accept header strictly prefers a text media type over JSON.
// It always sets "Vary: Accept" on the response and never returns 406 —
// when negotiation doesn't apply (or is ambiguous), the JSON route wins.
func NegotiateText(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept")
		if r.Method == http.MethodGet && prefersText(r.Header.Get("Accept")) && hasTxtTwin(r.URL.Path) {
			r.URL.Path += ".txt"
		}
		next.ServeHTTP(w, r)
	})
}

// hasTxtTwin reports whether path matches a JSON route known to have a
// ".txt" twin.
func hasTxtTwin(path string) bool {
	if strings.HasSuffix(path, ".txt") {
		return false
	}
	for _, re := range txtTwins {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

// prefersText reports whether the Accept header strictly prefers a text
// media type (text/plain, text/markdown, or text/*) over JSON (application/json,
// application/*, or */*). Ties, an empty/absent header, or JSON winning all
// resolve to false so a plain curl (no Accept, or "*/*") always gets JSON.
func prefersText(accept string) bool {
	if accept == "" {
		return false
	}

	var textQ, jsonQ float64

	for _, part := range strings.Split(accept, ",") {
		mediaType, q := parseMediaRange(part)
		if mediaType == "" {
			continue
		}

		switch {
		case mediaType == "text/plain", mediaType == "text/markdown", mediaType == "text/*":
			if q > textQ {
				textQ = q
			}
		case mediaType == "application/json", strings.HasPrefix(mediaType, "application/"), mediaType == "*/*":
			if q > jsonQ {
				jsonQ = q
			}
		}
	}

	return textQ > jsonQ
}

// parseMediaRange parses a single comma-separated Accept segment (e.g.
// " text/plain ; q=0.8 ") into its lowercased media type and q-value.
// A missing or malformed q defaults to 1. Parameters other than q are
// ignored.
func parseMediaRange(part string) (mediaType string, q float64) {
	q = 1
	segments := strings.Split(part, ";")

	mediaType = strings.ToLower(strings.TrimSpace(segments[0]))
	if mediaType == "" {
		return "", 0
	}

	for _, param := range segments[1:] {
		name, value, ok := strings.Cut(param, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(strings.ToLower(name)) != "q" {
			continue
		}
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			q = parsed
		}
		// Malformed q values fall back to the default of 1.
	}

	return mediaType, q
}
