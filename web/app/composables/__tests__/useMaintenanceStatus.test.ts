import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { computeMaintenanceStatus, THRESHOLDS } from "../useMaintenanceStatus";

describe("computeMaintenanceStatus", () => {
  const NOW = new Date("2026-04-13T00:00:00Z");

  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  function daysAgo(days: number): string {
    const d = new Date(NOW);
    d.setDate(d.getDate() - days);
    return d.toISOString();
  }

  it("returns undefined for active package with repo", () => {
    const result = computeMaintenanceStatus(daysAgo(30), daysAgo(15), false);
    expect(result).toBeUndefined();
  });

  it("returns possibly_unmaintained when both thresholds exceeded", () => {
    const result = computeMaintenanceStatus(
      daysAgo(THRESHOLDS.POSSIBLY_UNMAINTAINED_RELEASE_DAYS + 1),
      daysAgo(THRESHOLDS.POSSIBLY_UNMAINTAINED_COMMIT_DAYS + 1),
      false,
    );
    expect(result).toBe("possibly_unmaintained");
  });

  it("returns likely_unmaintained when both high thresholds exceeded", () => {
    const result = computeMaintenanceStatus(
      daysAgo(THRESHOLDS.LIKELY_UNMAINTAINED_RELEASE_DAYS + 1),
      daysAgo(THRESHOLDS.LIKELY_UNMAINTAINED_COMMIT_DAYS + 1),
      false,
    );
    expect(result).toBe("likely_unmaintained");
  });

  it("returns likely_unmaintained immediately for archived repos", () => {
    const result = computeMaintenanceStatus(daysAgo(30), daysAgo(15), true);
    expect(result).toBe("likely_unmaintained");
  });

  it("returns undefined for no-repo active package", () => {
    const result = computeMaintenanceStatus(daysAgo(30), undefined, false);
    expect(result).toBeUndefined();
  });

  it("returns possibly_unmaintained for no-repo old release", () => {
    const result = computeMaintenanceStatus(
      daysAgo(THRESHOLDS.POSSIBLY_UNMAINTAINED_NO_REPO_DAYS + 1),
      undefined,
      false,
    );
    expect(result).toBe("possibly_unmaintained");
  });

  it("returns likely_unmaintained for no-repo very old release", () => {
    const result = computeMaintenanceStatus(
      daysAgo(THRESHOLDS.LIKELY_UNMAINTAINED_NO_REPO_DAYS + 1),
      undefined,
      false,
    );
    expect(result).toBe("likely_unmaintained");
  });

  it("returns undefined when release is old but commit is recent", () => {
    const result = computeMaintenanceStatus(
      daysAgo(THRESHOLDS.POSSIBLY_UNMAINTAINED_RELEASE_DAYS + 1),
      daysAgo(15),
      false,
    );
    expect(result).toBeUndefined();
  });

  it("returns undefined when commit is old but release is recent", () => {
    const result = computeMaintenanceStatus(
      daysAgo(30),
      daysAgo(THRESHOLDS.POSSIBLY_UNMAINTAINED_COMMIT_DAYS + 1),
      false,
    );
    expect(result).toBeUndefined();
  });
});
