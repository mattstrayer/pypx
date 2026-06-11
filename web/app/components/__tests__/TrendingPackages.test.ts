import { describe, it, expect } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import TrendingPackages from "../TrendingPackages.vue";
import type { SearchResult } from "~/types/api";

function makePackage(name: string, downloads: number): SearchResult {
  return { name, summary: `Summary for ${name}`, downloads };
}

const packages: SearchResult[] = [
  makePackage("popular", 1_000_000),
  makePackage("medium", 1_000),
  makePackage("rare", 1),
];

describe("TrendingPackages", () => {
  it("renders all three package names", async () => {
    const wrapper = await mountSuspended(TrendingPackages, {
      props: { packages },
    });

    const text = wrapper.text();
    expect(text).toContain("popular");
    expect(text).toContain("medium");
    expect(text).toContain("rare");
  });

  it("renders the correct number of list items", async () => {
    const wrapper = await mountSuspended(TrendingPackages, {
      props: { packages },
    });

    const items = wrapper.findAll("li");
    expect(items).toHaveLength(3);
  });

  it("highest-download package has the most filled bar cells", async () => {
    const wrapper = await mountSuspended(TrendingPackages, {
      props: { packages },
    });

    const items = wrapper.findAll("li");
    expect(items.length).toBeGreaterThanOrEqual(3);

    // The ASCII bar filled spans have the opacity-80 class (brand-colored cells)
    function countFilledBars(item: ReturnType<typeof wrapper.findAll>[0]): number {
      return item.findAll("span.opacity-80").length;
    }

    const popularBars = countFilledBars(items[0]!);
    const mediumBars = countFilledBars(items[1]!);
    const rareBars = countFilledBars(items[2]!);

    // popular (1M downloads) must have more filled bars than medium (1K) and rare (1)
    expect(popularBars).toBeGreaterThan(mediumBars);
    expect(mediumBars).toBeGreaterThan(rareBars);
  });

  it("renders rank numbers starting from 01.", async () => {
    const wrapper = await mountSuspended(TrendingPackages, {
      props: { packages },
    });

    const text = wrapper.text();
    expect(text).toContain("01.");
    expect(text).toContain("02.");
    expect(text).toContain("03.");
  });

  it("renders download counts with /mo suffix", async () => {
    const wrapper = await mountSuspended(TrendingPackages, {
      props: { packages },
    });

    const text = wrapper.text();
    expect(text).toMatch(/\/mo/);
  });
});
