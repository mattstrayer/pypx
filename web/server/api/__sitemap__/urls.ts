import { defineSitemapEventHandler } from "#imports";
import type { SitemapUrlInput } from "#sitemap/types";

export default defineSitemapEventHandler(async () => {
  const config = useRuntimeConfig();
  const apiBase = (config.apiBase as string) || "http://localhost:8080";

  const [popularRes, cachedRes] = await Promise.allSettled([
    $fetch<{ packages: string[] }>(`${apiBase}/api/sitemap/popular`),
    $fetch<{ packages: string[] }>(`${apiBase}/api/sitemap/cached`),
  ]);

  const popularNames = new Set<string>(
    popularRes.status === "fulfilled" ? (popularRes.value.packages ?? []) : [],
  );
  const cachedNames: string[] =
    cachedRes.status === "fulfilled" ? (cachedRes.value.packages ?? []) : [];

  const urls: SitemapUrlInput[] = [];

  for (const name of popularNames) {
    urls.push({
      loc: `/packages/${name}`,
      changefreq: "daily",
      priority: 1.0,
      _sitemap: "popular",
    });
  }

  for (const name of cachedNames) {
    if (!popularNames.has(name)) {
      urls.push({
        loc: `/packages/${name}`,
        changefreq: "weekly",
        priority: 0.5,
        _sitemap: "cached",
      });
    }
  }

  return urls;
});
