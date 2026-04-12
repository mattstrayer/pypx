package enrichment_test

import (
	"testing"

	"github.com/pypx/api/internal/enrichment"
	"github.com/pypx/api/internal/pypi"
)

func TestComputeReleaseCadence(t *testing.T) {
	releases := map[string][]pypi.ReleaseFile{
		"1.0.0": {{UploadTime: "2024-01-15T10:00:00Z", PackageType: "bdist_wheel"}},
		"1.1.0": {{UploadTime: "2024-04-20T10:00:00Z", PackageType: "bdist_wheel"}},
		"1.2.0": {{UploadTime: "2024-07-10T10:00:00Z", PackageType: "bdist_wheel"}},
		"2.0.0": {{UploadTime: "2025-10-01T10:00:00Z", PackageType: "bdist_wheel"}},
		"2.1.0": {{UploadTime: "2026-03-01T10:00:00Z", PackageType: "bdist_wheel"}},
	}

	cadence := enrichment.ComputeReleaseCadence(releases)

	if cadence.LastReleasedAt == "" {
		t.Error("LastReleasedAt should not be empty")
	}
	if cadence.ReleasesLast12Mo < 1 {
		t.Errorf("ReleasesLast12Mo = %d, want >= 1", cadence.ReleasesLast12Mo)
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
