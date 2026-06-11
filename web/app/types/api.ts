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
