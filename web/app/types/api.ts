// Hand-written shim: all API contract types are generated from the Go structs.
// Run `cd api && go run ./cmd/gentypes -out ../web/app/types/api.gen.ts` after
// changing a response struct, then commit the updated api.gen.ts.

// Re-export all generated types under their canonical Go names.
export * from "./api.gen";

// Re-export with the original frontend names (Go type names differ).
export type { PackageResponse as PackageData } from "./api.gen";
export type { CombinedStats as StatsData } from "./api.gen";
export type { PackageEntry as SearchResult } from "./api.gen";
export type { Entry as ChangelogEntry } from "./api.gen";
export type { ChangelogResponse as ChangelogData } from "./api.gen";
export type { SecurityResponse as SecurityData } from "./api.gen";
export type { ExtrasResponse as ExtrasData } from "./api.gen";
export type { DocsResponse as DocsData } from "./api.gen";

// --- Compare / diff types ---
// Frontend-only: these mirror textfmt.CompareInput / textfmt.DiffInput, which
// are not part of the generated contract (the compare/diff JSON routes expose
// the textfmt input structs directly). If those structs change, update these by
// hand — they are not covered by the gentypes drift gate.

// CompareData mirrors textfmt.CompareInput (JSON response from GET /api/compare).
export interface ComparePackageData {
  Name: string;
  Version: string;
  Summary: string;
  License: string;
  PythonMin: string;
  InstallSize: number;
  ModuleFormat: string;
  LastReleasedDate: string;
  ReleasesLast12Mo: number;
  DepCount: number;
  Downloads30d: number;
  VulnCount: number;
  Typed: string;
  RepoURL: string;
  DocURL: string;
}

export interface SkippedPackage {
  Name: string;
  Reason: string;
}

export interface CompareData {
  Skipped: SkippedPackage[] | null;
  Packages: ComparePackageData[] | null;
}

// DiffData mirrors textfmt.DiffInput (JSON response from GET /api/packages/{name}/diff).
export interface DepBump {
  Name: string;
  FromConstraint: string;
  ToConstraint: string;
}

export interface DepDiff {
  Added: string[] | null;
  Removed: string[] | null;
  Bumped: DepBump[] | null;
}

export interface APIDiffChange {
  Path: string;
  FromSig: string;
  ToSig: string;
}

export interface APIDiff {
  Added: string[] | null;
  Removed: string[] | null;
  Changed: APIDiffChange[] | null;
  AddedTruncated: number;
  RemovedTruncated: number;
  ChangedTruncated: number;
}

export interface DiffChangelogEntry {
  version: string;
  tag_name: string;
  title: string;
  body: string;
  body_html: string;
  published_at: string;
  url: string;
}

export interface DiffData {
  Package: string;
  From: string;
  To: string;
  Changelog: DiffChangelogEntry[] | null;
  ChangelogUnavailable: string;
  DepChanges: DepDiff;
  DepChangesUnavailable: string;
  APIChanges: APIDiff;
  APIChangesUnavailable: string;
}
