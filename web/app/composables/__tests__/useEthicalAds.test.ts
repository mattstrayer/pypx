import { describe, it, expect, beforeEach, vi } from "vitest";
import { mockNuxtImport } from "@nuxt/test-utils/runtime";
import { useEthicalAds } from "../useEthicalAds";

const mockConfig = {
  app: { baseURL: "/" },
  public: { ads: { publisher: "", type: "image" } },
};

mockNuxtImport("useRuntimeConfig", () => () => mockConfig);

const headEntries: unknown[] = [];
mockNuxtImport("useHead", () => (entry: unknown) => {
  headEntries.push(entry);
});

describe("useEthicalAds", () => {
  beforeEach(() => {
    mockConfig.public.ads = { publisher: "", type: "image" };
    headEntries.length = 0;
    delete (window as unknown as Record<string, unknown>).ethicalads;
  });

  it("is disabled and injects no script when no publisher is configured", () => {
    const ads = useEthicalAds();
    expect(ads.enabled).toBe(false);
    expect(headEntries).toHaveLength(0);
  });

  it("injects the client script when a publisher is configured", () => {
    mockConfig.public.ads.publisher = "pypx";
    useEthicalAds();
    expect(headEntries).toHaveLength(1);
  });

  it("exposes the configured publisher and type", () => {
    mockConfig.public.ads = { publisher: "pypx", type: "text" };
    const ads = useEthicalAds();
    expect(ads.publisher).toBe("pypx");
    expect(ads.type).toBe("text");
  });

  it("delegates reload to the client when present", () => {
    mockConfig.public.ads.publisher = "pypx";
    const reload = vi.fn();
    (window as unknown as Record<string, unknown>).ethicalads = { reload };
    useEthicalAds().reload();
    expect(reload).toHaveBeenCalledOnce();
  });

  it("does not throw when reload is called and the client is absent", () => {
    mockConfig.public.ads.publisher = "pypx";
    expect(() => useEthicalAds().reload()).not.toThrow();
  });
});
