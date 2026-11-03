import type {
  PackageData,
  VersionInfo,
  DependencyTree,
  StatsData,
  SearchResult,
  ChangelogData,
  SecurityData,
  ExtrasData,
  DocsData,
  CompareData,
  DiffData,
} from "~/types/api";

export function useApi() {
  const config = useRuntimeConfig();
  // Server-side: use private apiBase (http://api:8080 inside Docker network).
  // Client-side: use public apiBase (relative /api path proxied by Nuxt dev server or Caddy).
  const baseURL = import.meta.server
    ? (config.apiBase as string) || config.public.apiBase
    : config.public.apiBase;

  async function request<T>(path: string, params?: Record<string, unknown>): Promise<T> {
    try {
      return await $fetch<T>(`${baseURL}${path}`, params ? { params } : {});
    } catch (err) {
      if (!import.meta.server) throw err;
      // The full error, with its URL, stays in the server log.
      // oxlint-disable-next-line no-console
      console.error(`[api] ${path}:`, err);
      throw toPublicApiError(err);
    }
  }

  async function fetchPackage(name: string): Promise<PackageData> {
    return request(`/packages/${name}`);
  }

  async function fetchVersions(name: string): Promise<VersionInfo[]> {
    return request(`/packages/${name}/versions`);
  }

  async function fetchDependencies(name: string): Promise<DependencyTree> {
    return request(`/packages/${name}/dependencies`);
  }

  async function fetchStats(name: string, period?: string): Promise<StatsData> {
    return request(`/packages/${name}/stats`, period ? { period } : undefined);
  }

  async function searchPackages(query: string, limit = 20): Promise<SearchResult[]> {
    return request("/search", { q: query, limit });
  }

  async function fetchChangelog(name: string): Promise<ChangelogData> {
    return request(`/packages/${name}/changelog`);
  }

  async function fetchSecurity(name: string, version?: string): Promise<SecurityData> {
    return request(`/packages/${name}/security`, version ? { version } : undefined);
  }

  async function fetchExtras(name: string): Promise<ExtrasData> {
    return request(`/packages/${name}/extras`);
  }

  async function fetchDocs(name: string): Promise<DocsData> {
    return request(`/packages/${name}/docs`);
  }

  async function fetchPopular(
    limit = 24,
  ): Promise<Array<{ name: string; summary: string; downloads: number }>> {
    return request("/popular", { limit });
  }

  async function fetchCompare(pkgs: string[]): Promise<CompareData> {
    return request("/compare", { pkgs: pkgs.join(",") });
  }

  async function fetchDiff(name: string, from: string, to: string): Promise<DiffData> {
    return request(`/packages/${name}/diff`, { from, to });
  }

  return {
    fetchPackage,
    fetchVersions,
    fetchDependencies,
    fetchStats,
    searchPackages,
    fetchChangelog,
    fetchSecurity,
    fetchExtras,
    fetchDocs,
    fetchPopular,
    fetchCompare,
    fetchDiff,
  };
}
