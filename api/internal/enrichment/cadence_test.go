package enrichment_test

import (
	"testing"
	"time"

	"github.com/pypx/api/internal/enrichment"
	"github.com/pypx/api/internal/pypi"
)

func TestComputeReleaseCadence(t *testing.T) {
	now := time.Now()
	releases := map[string][]pypi.ReleaseFile{
		"1.0.0": {{UploadTime: now.AddDate(-2, 6, 0).UTC().Format(time.RFC3339), PackageType: "bdist_wheel"}},
		"1.1.0": {{UploadTime: now.AddDate(-1, -9, 0).UTC().Format(time.RFC3339), PackageType: "bdist_wheel"}},
		"2.0.0": {{UploadTime: now.AddDate(0, -6, 0).UTC().Format(time.RFC3339), PackageType: "bdist_wheel"}},
		"2.1.0": {{UploadTime: now.AddDate(0, -1, 0).UTC().Format(time.RFC3339), PackageType: "bdist_wheel"}},
	}

	cadence := enrichment.ComputeReleaseCadence(releases)

	if cadence.LastReleasedAt == "" {
		t.Error("LastReleasedAt should not be empty")
	}
	// 2.0.0 (6 months ago) and 2.1.0 (1 month ago) are within 12 months
	if cadence.ReleasesLast12Mo < 2 {
		t.Errorf("ReleasesLast12Mo = %d, want >= 2", cadence.ReleasesLast12Mo)
	}
	if cadence.AvgDaysBetweenReleases <= 0 {
		t.Errorf("AvgDaysBetweenReleases = %f, want > 0", cadence.AvgDaysBetweenReleases)
	}
	if len(cadence.QuarterlyCounts) == 0 {
		t.Error("QuarterlyCounts should not be empty")
	}
}

func TestComputeReleaseCadenceEmpty(t *testing.T) {
	cadence := enrichment.ComputeReleaseCadence(map[string][]pypi.ReleaseFile{})
	if cadence.LastReleasedAt != "" {
		t.Errorf("expected empty LastReleasedAt for empty releases, got %q", cadence.LastReleasedAt)
	}
	if cadence.AvgDaysBetweenReleases != 0 {
		t.Errorf("expected 0 avg days for empty releases, got %f", cadence.AvgDaysBetweenReleases)
	}
}

func TestComputeReleaseCadenceSingleRelease(t *testing.T) {
	releases := map[string][]pypi.ReleaseFile{
		"1.0.0": {{UploadTime: "2025-01-01T10:00:00Z", PackageType: "bdist_wheel"}},
	}
	cadence := enrichment.ComputeReleaseCadence(releases)
	if cadence.AvgDaysBetweenReleases != 0 {
		t.Errorf("single release: avg days should be 0, got %f", cadence.AvgDaysBetweenReleases)
	}
}
