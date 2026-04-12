import type {
  PackageData,
  VersionInfo,
  DependencyTree,
  StatsData,
  SearchResult,
  ChangelogData,
  SecurityData,
  ExtrasData,
} from "~/types/api";

export function useApi() {
  const config = useRuntimeConfig();
  const baseURL = config.public.apiBase;

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

  async function fetchSecurity(name: string): Promise<SecurityData> {
    return $fetch<SecurityData>(`${baseURL}/packages/${name}/security`);
  }

  async function fetchExtras(name: string): Promise<ExtrasData> {
    return $fetch<ExtrasData>(`${baseURL}/packages/${name}/extras`);
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
  };
}
