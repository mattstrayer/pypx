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
} from "~/types/api";

export function useApi() {
  const config = useRuntimeConfig();
  // Server-side: use private apiBase (http://api:8080 inside Docker network).
  // Client-side: fall back to public apiBase (http://localhost:8080/api from browser).
  const baseURL = (config.apiBase as string) || config.public.apiBase;

  async function fetchPackage(name: string): Promise<PackageData> {
    return $fetch<PackageData>(`${baseURL}/packages/${name}`);
  }

  async function fetchVersions(name: string): Promise<VersionInfo[]> {
    return $fetch<VersionInfo[]>(`${baseURL}/packages/${name}/versions`);
  }

  async function fetchDependencies(name: string): Promise<DependencyTree> {
    return $fetch<DependencyTree>(`${baseURL}/packages/${name}/dependencies`);
  }

  async function fetchStats(name: string, period?: string): Promise<StatsData> {
    return $fetch<StatsData>(`${baseURL}/packages/${name}/stats`, {
      params: period ? { period } : undefined,
    });
  }

  async function searchPackages(query: string, limit = 20): Promise<SearchResult[]> {
    return $fetch<SearchResult[]>(`${baseURL}/search`, {
      params: { q: query, limit },
    });
  }

  async function fetchChangelog(name: string): Promise<ChangelogData> {
    return $fetch<ChangelogData>(`${baseURL}/packages/${name}/changelog`);
  }

  async function fetchSecurity(name: string, version?: string): Promise<SecurityData> {
    return $fetch<SecurityData>(`${baseURL}/packages/${name}/security`, {
      params: version ? { version } : undefined,
    });
  }

  async function fetchExtras(name: string): Promise<ExtrasData> {
    return $fetch<ExtrasData>(`${baseURL}/packages/${name}/extras`);
  }

  async function fetchDocs(name: string): Promise<DocsData> {
    return $fetch<DocsData>(`${baseURL}/packages/${name}/docs`);
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
  };
}
