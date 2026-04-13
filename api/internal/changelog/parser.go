package changelog

import (
	"regexp"
	"strings"
)

type headingPattern struct {
	re           *regexp.Regexp
	versionGroup int
	dateGroup    int
}

var versionRE = regexp.MustCompile(`^\d+\.\d[\d.a-zA-Z\-]*$`)
var noiseVersionRE = regexp.MustCompile(`(?i)^unreleased$`)

var mdPatterns = []headingPattern{
	// ## [1.0.0] - 2023-06-20  (Keep a Changelog, git-cliff, release-please)
	{
		re:           regexp.MustCompile(`^#{1,3}\s+\[v?([0-9]+[0-9a-zA-Z.\-]*)\]\s*-\s*(\d{4}-\d{2}-\d{2})`),
		versionGroup: 1,
		dateGroup:    2,
	},
	// ## [v1.17.0](url) (2026-03-13)  (github-changelog-generator)
	{
		re:           regexp.MustCompile(`^#{1,3}\s+\[v?([0-9]+[0-9a-zA-Z.\-]*)\](?:\([^)]*\))?\s*\((\d{4}-\d{2}-\d{2})\)`),
		versionGroup: 1,
		dateGroup:    2,
	},
	// ## 2.31.0 (2026-04-08)  (OpenAI SDK, requests HISTORY.md)
	{
		re:           regexp.MustCompile(`^#{1,3}\s+v?([0-9]+[0-9a-zA-Z.\-]*)\s+\((\d{4}-\d{2}-\d{2})\)\s*$`),
		versionGroup: 1,
		dateGroup:    2,
	},
	// ## [1.0.0]  (bare bracket, no date)
	{
		re:           regexp.MustCompile(`^#{1,3}\s+\[v?([0-9]+[0-9a-zA-Z.\-]*)\]\s*$`),
		versionGroup: 1,
		dateGroup:    0,
	},
	// ## v1.0.0 - 2024-01-01  or  ## v1.0.0  (bare heading with optional date after dash)
	{
		re:           regexp.MustCompile(`^#{1,3}\s+v?([0-9]+\.[0-9][0-9a-zA-Z.\-]*)\s*(?:-\s*(\d{4}-\d{2}-\d{2}))?\s*$`),
		versionGroup: 1,
		dateGroup:    2,
	},
}

type parsedSection struct {
	version string
	tagName string
	date    string
	body    []string
}

// Parse parses a CHANGELOG file into entries. Returns empty slice if fewer than
// 2 version sections are found (strict mode).
func Parse(content string) []Entry {
	lines := strings.Split(content, "\n")

	sections := parseMarkdown(lines)
	if len(sections) < 2 {
		sections = parseRST(lines)
	}
	if len(sections) < 2 {
		return nil
	}

	entries := make([]Entry, 0, len(sections))
	for _, s := range sections {
		body := strings.TrimSpace(strings.Join(s.body, "\n"))
		entries = append(entries, Entry{
			Version:     s.version,
			TagName:     s.tagName,
			Title:       s.tagName,
			Body:        body,
			PublishedAt: s.date,
		})
	}
	return entries
}

func parseMarkdown(lines []string) []parsedSection {
	var sections []parsedSection
	var current *parsedSection

	flush := func() {
		if current != nil {
			sections = append(sections, *current)
			current = nil
		}
	}

	for _, line := range lines {
		if version, date, tag, ok := matchMDHeading(line); ok {
			flush()
			if noiseVersionRE.MatchString(version) {
				current = nil
				continue
			}
			current = &parsedSection{version: version, tagName: tag, date: date}
			continue
		}
		if current != nil {
			current.body = append(current.body, line)
		}
	}
	flush()
	return sections
}

func matchMDHeading(line string) (version, date, tag string, ok bool) {
	for _, p := range mdPatterns {
		m := p.re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		raw := m[p.versionGroup]
		if !versionRE.MatchString(strings.TrimPrefix(raw, "v")) {
			if noiseVersionRE.MatchString(raw) {
				return raw, "", raw, true
			}
			continue
		}
		normalized := strings.TrimPrefix(raw, "v")
		d := ""
		if p.dateGroup > 0 && p.dateGroup < len(m) {
			d = m[p.dateGroup]
		}
		// For TagName: if raw doesn't have 'v', but the version does start with 'v' in the original line,
		// add the v prefix to preserve it. Otherwise use raw as-is.
		tagName := raw
		if !strings.HasPrefix(raw, "v") && strings.Contains(line, "v"+raw) {
			tagName = "v" + raw
		}
		return normalized, d, tagName, true
	}
	return "", "", "", false
}

var rstVersionPrefixed = regexp.MustCompile(`^Version\s+(v?[0-9]+\.[0-9][0-9a-zA-Z.\-]*)\s*$`)
var rstVersionBare = regexp.MustCompile(`^(v?[0-9]+\.[0-9][0-9a-zA-Z.\-]*)\s*$`)
var rstUnderline = regexp.MustCompile(`^[-=~^#*+]{3,}\s*$`)
var rstDateLine = regexp.MustCompile(`(?:Released|Date:)\s*(\d{4}-\d{2}-\d{2})`)

func parseRST(lines []string) []parsedSection {
	var sections []parsedSection
	var current *parsedSection

	flush := func() {
		if current != nil {
			sections = append(sections, *current)
			current = nil
		}
	}

	for i, line := range lines {
		if i+1 < len(lines) && rstUnderline.MatchString(lines[i+1]) {
			var raw string
			if m := rstVersionPrefixed.FindStringSubmatch(line); m != nil {
				raw = m[1]
			} else if m := rstVersionBare.FindStringSubmatch(line); m != nil {
				raw = m[1]
			}
			if raw != "" {
				normalized := strings.TrimPrefix(raw, "v")
				flush()
				current = &parsedSection{version: normalized, tagName: raw}
				continue
			}
		}
		if current != nil {
			if len(current.body) < 4 {
				if m := rstDateLine.FindStringSubmatch(line); m != nil {
					current.date = m[1]
					continue
				}
			}
			current.body = append(current.body, line)
		}
	}
	flush()
	return sections
}
