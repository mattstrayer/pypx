import { describe, it, expect, beforeEach } from "vitest";
import { mockNuxtImport } from "@nuxt/test-utils/runtime";
import ethicalAdsPlugin from "../ethicalads.client";
import { ETHICALADS_CLIENT_SRC } from "../../composables/useEthicalAds";

const mockConfig = {
  app: { baseURL: "/" },
  public: { ads: { publisher: "", type: "image" } },
};

mockNuxtImport("useRuntimeConfig", () => () => mockConfig);

interface HeadEntry {
  script?: { src?: string; async?: boolean; key?: string }[];
}

const headEntries: HeadEntry[] = [];
mockNuxtImport("useHead", () => (entry: HeadEntry) => {
  headEntries.push(entry);
});

function runPlugin(): void {
  const setup =
    (ethicalAdsPlugin as unknown as { setup?: (app: unknown) => void }).setup ??
    (ethicalAdsPlugin as unknown as (app: unknown) => void);
  setup({ provide: () => {} });
}

describe("ethicalads.client plugin", () => {
  beforeEach(() => {
    mockConfig.public.ads = { publisher: "", type: "image" };
    headEntries.length = 0;
  });

  it("injects no script when no publisher is configured", () => {
    runPlugin();
    expect(headEntries).toHaveLength(0);
  });

  it("injects the vendor client script once when a publisher is configured", () => {
    mockConfig.public.ads.publisher = "pypx";
    runPlugin();

    expect(headEntries).toHaveLength(1);
    const script = headEntries[0]?.script?.[0];
    expect(script?.src).toBe(ETHICALADS_CLIENT_SRC);
    expect(script?.async).toBe(true);
    expect(script?.key).toBe("ethicalads");
  });
});
