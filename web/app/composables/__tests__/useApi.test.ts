import { describe, it, expect, vi, afterEach } from "vitest";
import { useApi } from "../useApi";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useApi", () => {
  describe("fetchPackage", () => {
    it("calls $fetch with URL ending /packages/requests and returns payload", async () => {
      const mockFetch = vi.fn().mockResolvedValue({ name: "requests" });
      vi.stubGlobal("$fetch", mockFetch);

      const { fetchPackage } = useApi();
      const result = await fetchPackage("requests");

      expect(mockFetch).toHaveBeenCalledOnce();
      const [url] = mockFetch.mock.calls[0] as [string, ...unknown[]];
      expect(url).toMatch(/\/packages\/requests$/);
      expect(result).toEqual({ name: "requests" });
    });
  });

  describe("searchPackages", () => {
    it("passes q and limit params", async () => {
      const mockFetch = vi.fn().mockResolvedValue([]);
      vi.stubGlobal("$fetch", mockFetch);

      const { searchPackages } = useApi();
      await searchPackages("http", 5);

      expect(mockFetch).toHaveBeenCalledOnce();
      const [url, opts] = mockFetch.mock.calls[0] as [string, { params: Record<string, unknown> }];
      expect(url).toMatch(/\/search$/);
      expect(opts.params).toEqual({ q: "http", limit: 5 });
    });
  });

  describe("fetchStats", () => {
    it("omits params when no period given", async () => {
      const mockFetch = vi.fn().mockResolvedValue({});
      vi.stubGlobal("$fetch", mockFetch);

      const { fetchStats } = useApi();
      await fetchStats("requests");

      const [, opts] = mockFetch.mock.calls[0] as [string, { params: unknown }];
      expect(opts.params).toBeUndefined();
    });

    it("passes period param when provided", async () => {
      const mockFetch = vi.fn().mockResolvedValue({});
      vi.stubGlobal("$fetch", mockFetch);

      const { fetchStats } = useApi();
      await fetchStats("requests", "4w");

      const [, opts] = mockFetch.mock.calls[0] as [string, { params: Record<string, unknown> }];
      expect(opts.params).toEqual({ period: "4w" });
    });
  });

  describe("error propagation", () => {
    it("rejects when $fetch rejects (useApi does NOT swallow errors)", async () => {
      const mockFetch = vi.fn().mockRejectedValue(new Error("network error"));
      vi.stubGlobal("$fetch", mockFetch);

      const { fetchPackage } = useApi();
      await expect(fetchPackage("requests")).rejects.toThrow("network error");
    });
  });

  describe("baseURL selection", () => {
    it("uses public.apiBase on the client side (import.meta.server is false in test env)", async () => {
      const mockFetch = vi.fn().mockResolvedValue({ name: "requests" });
      vi.stubGlobal("$fetch", mockFetch);

      const { fetchPackage } = useApi();
      await fetchPackage("requests");

      const [url] = mockFetch.mock.calls[0] as [string, ...unknown[]];
      // In the nuxt test environment, import.meta.server is false so it should
      // use config.public.apiBase ("/api"), not the server-only apiBase.
      expect(url).toMatch(/^\/api\//);
    });
  });
});
