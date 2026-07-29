package middleware

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// txtTwins lists the JSON routes that have a plain-text ".txt" twin.
// {name} segments never contain "/" (PEP 503 normalized names).
//
// Note: there is deliberately no entry for /api/packages/{name}/docs/{symbol}.
// That route has no .txt twin — main.go registers only the JSON route, whose
// handler is text-only and strips a ".txt" suffix off the symbol itself.
// Matching it here would be a no-op today by luck (nothing appends real
// content after the rewrite) but a latent trap the moment a real twin route
// is added with different semantics.
var txtTwins = []*regexp.Regexp{
	regexp.MustCompile(`^/api/packages/[^/]+$`),
	regexp.MustCompile(`^/api/packages/[^/]+/(changelog|security|extras|stats|docs|diff)$`),
	regexp.MustCompile(`^/api/(search|compare|popular)$`),
}

// NegotiateText rewrites the request path to its ".txt" twin when the
// client's Accept header strictly prefers a text media type over JSON.
// It always sets "Vary: Accept" on the response and never returns 406 —
// when negotiation doesn't apply (or is ambiguous), the JSON route wins.
//
// The rate limiter is deliberately mounted before this middleware (see
// main.go), so requests it rejects with 429 never reach here — those
// responses are neither negotiated (no .txt rewrite) nor tagged with
// "Vary: Accept". Do not reorder the two.
func NegotiateText(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept")
		if r.Method == http.MethodGet && prefersText(r.Header.Get("Accept")) && hasTxtTwin(r.URL.Path) {
			// chi (and net/http's ServeMux) route on URL.RawPath when it is
			// non-empty — which happens whenever the raw request target
			// contains percent-encoding that differs from its decoded form
			// (e.g. "%2D" vs "-"). Mutating only Path would silently leave
			// RawPath pointing at the un-rewritten route for those requests,
			// so both must be kept in sync.
			r.URL.Path += ".txt"
			if r.URL.RawPath != "" {
				r.URL.RawPath += ".txt"
			}
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
		// Any application/* subtype (not just application/json) counts as a
		// JSON vote — deliberately broad, since our JSON responses are the
		// only thing under application/* we ever serve, and a client naming
		// e.g. application/pdf still isn't asking for text/plain.
		case strings.HasPrefix(mediaType, "application/"), mediaType == "*/*":
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

	// Clamp to the valid RFC 7231 range [0, 1]: an out-of-range q (e.g. a
	// bogus "q=2") must never let one media type out-vote a
	// legitimately-weighted competitor.
	if q > 1 {
		q = 1
	} else if q < 0 {
		q = 0
	}

	return mediaType, q
}
