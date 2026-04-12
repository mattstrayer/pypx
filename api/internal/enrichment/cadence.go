package enrichment

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/pypx/api/internal/pypi"
)

// ReleaseCadence summarises how often a package publishes new versions.
type ReleaseCadence struct {
	ReleasesLast12Mo       int            `json:"releases_last_12mo"`
	AvgDaysBetweenReleases float64        `json:"avg_days_between_releases"`
	LastReleasedAt         string         `json:"last_released_at"`
	QuarterlyCounts        []QuarterCount `json:"quarterly_counts"`
}

// QuarterCount holds the number of releases in a calendar quarter.
type QuarterCount struct {
	Quarter string `json:"quarter"` // e.g. "2025 Q1"
	Count   int    `json:"count"`
}

// ComputeReleaseCadence derives release frequency from all version files.
// Versions with no parseable upload_time are silently skipped.
func ComputeReleaseCadence(releases map[string][]pypi.ReleaseFile) ReleaseCadence {
	// Collect one timestamp per version (first parseable file wins).
	var times []time.Time
	for _, files := range releases {
		for _, f := range files {
			if f.UploadTime == "" {
				continue
			}
			t, err := time.Parse(time.RFC3339Nano, f.UploadTime)
			if err != nil {
				t, err = time.Parse(time.RFC3339, f.UploadTime)
			}
			if err == nil {
				times = append(times, t)
				break
			}
		}
	}

	if len(times) == 0 {
		return ReleaseCadence{}
	}

	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })

	now := time.Now()
	oneYearAgo := now.AddDate(-1, 0, 0)
	twoYearsAgo := now.AddDate(-2, 0, 0)

	var cadence ReleaseCadence
	cadence.LastReleasedAt = times[len(times)-1].UTC().Format(time.RFC3339)

	for _, t := range times {
		if !t.Before(oneYearAgo) {
			cadence.ReleasesLast12Mo++
		}
	}

	if len(times) >= 2 {
		totalDays := times[len(times)-1].Sub(times[0]).Hours() / 24
		avg := totalDays / float64(len(times)-1)
		cadence.AvgDaysBetweenReleases = math.Round(avg*10) / 10
	}

	// Build quarterly counts covering the last 2 years, in chronological order.
	quarterStart := startOfQuarter(twoYearsAgo)
	quarterMap := make(map[string]int)
	for _, t := range times {
		if t.Before(quarterStart) {
			continue
		}
		q := quarterLabel(t)
		quarterMap[q]++
	}

	for t := quarterStart; !t.After(now); t = t.AddDate(0, 3, 0) {
		q := quarterLabel(t)
		cadence.QuarterlyCounts = append(cadence.QuarterlyCounts, QuarterCount{
			Quarter: q,
			Count:   quarterMap[q],
		})
	}

	return cadence
}

func quarterLabel(t time.Time) string {
	q := (int(t.Month())-1)/3 + 1
	return fmt.Sprintf("%d Q%d", t.Year(), q)
}

func startOfQuarter(t time.Time) time.Time {
	month := ((int(t.Month()) - 1) / 3) * 3 + 1
	return time.Date(t.Year(), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
}
