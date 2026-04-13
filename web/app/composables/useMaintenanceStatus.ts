import type { PackageData, RepoInfo } from "~/types/api";

export type MaintenanceStatus = "possibly_unmaintained" | "likely_unmaintained";

export const THRESHOLDS = {
  POSSIBLY_UNMAINTAINED_RELEASE_DAYS: 548, // ~18 months
  POSSIBLY_UNMAINTAINED_COMMIT_DAYS: 365, // 12 months
  LIKELY_UNMAINTAINED_RELEASE_DAYS: 1095, // ~3 years
  LIKELY_UNMAINTAINED_COMMIT_DAYS: 730, // ~2 years
  POSSIBLY_UNMAINTAINED_NO_REPO_DAYS: 730, // 2 years (PyPI-only, conservative)
  LIKELY_UNMAINTAINED_NO_REPO_DAYS: 1095, // 3 years (PyPI-only)
} as const;

function daysBetween(dateStr: string): number {
  return (Date.now() - new Date(dateStr).getTime()) / (1000 * 60 * 60 * 24);
}

/**
 * Pure function that computes maintenance status from release and commit dates.
 * Returns undefined for active packages.
 */
export function computeMaintenanceStatus(
  lastReleasedAt: string | undefined,
  lastCommitAt: string | undefined,
  archived: boolean,
): MaintenanceStatus | undefined {
  if (!lastReleasedAt) return undefined;

  if (archived) return "likely_unmaintained";

  const releaseDays = daysBetween(lastReleasedAt);

  if (lastCommitAt !== undefined) {
    const commitDays = daysBetween(lastCommitAt);

    if (
      releaseDays >= THRESHOLDS.LIKELY_UNMAINTAINED_RELEASE_DAYS &&
      commitDays >= THRESHOLDS.LIKELY_UNMAINTAINED_COMMIT_DAYS
    ) {
      return "likely_unmaintained";
    }
    if (
      releaseDays >= THRESHOLDS.POSSIBLY_UNMAINTAINED_RELEASE_DAYS &&
      commitDays >= THRESHOLDS.POSSIBLY_UNMAINTAINED_COMMIT_DAYS
    ) {
      return "possibly_unmaintained";
    }
    return undefined;
  }

  // No repo: conservative PyPI-only thresholds
  if (releaseDays >= THRESHOLDS.LIKELY_UNMAINTAINED_NO_REPO_DAYS) {
    return "likely_unmaintained";
  }
  if (releaseDays >= THRESHOLDS.POSSIBLY_UNMAINTAINED_NO_REPO_DAYS) {
    return "possibly_unmaintained";
  }
  return undefined;
}

/**
 * Composable that returns a computed maintenance status for a package.
 */
export function useMaintenanceStatus(
  pkg: Ref<PackageData | null>,
  repoInfo: Ref<RepoInfo | null | undefined>,
) {
  return computed(() => {
    if (!pkg.value) return undefined;

    const lastReleasedAt = pkg.value.release_cadence?.last_released_at;
    const lastCommitAt = repoInfo.value?.last_pushed_at;
    const archived = repoInfo.value?.archived ?? false;

    return computeMaintenanceStatus(lastReleasedAt, lastCommitAt, archived);
  });
}
