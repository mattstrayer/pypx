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

  it("is disabled when no publisher is configured", () => {
    const ads = useEthicalAds();
    expect(ads.enabled).toBe(false);
  });

  it("is enabled when a publisher is configured", () => {
    mockConfig.public.ads.publisher = "pypx";
    expect(useEthicalAds().enabled).toBe(true);
  });

  // Script injection belongs to plugins/ethicalads.client.ts. A component-scoped
  // head entry is disposed when the component unmounts and re-injected on the
  // next mount, double-loading the vendor client.
  it("never injects the client script itself, publisher or not", () => {
    useEthicalAds();
    mockConfig.public.ads.publisher = "pypx";
    useEthicalAds();
    expect(headEntries).toHaveLength(0);
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
