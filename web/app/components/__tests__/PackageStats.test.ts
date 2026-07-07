import { describe, it, expect, vi } from "vitest";
import { mountSuspended, mockNuxtImport } from "@nuxt/test-utils/runtime";
import PackageStats from "../PackageStats.vue";

const fetchStats = vi.fn().mockResolvedValue({
  overall: [],
  python_versions: [],
  systems: [],
});

mockNuxtImport("useApi", () => () => ({ fetchStats }));

describe("PackageStats", () => {
  it("fetches stats on mount and refetches with the new period when the toggle is clicked", async () => {
    const wrapper = await mountSuspended(PackageStats, { props: { name: "requests" } });

    await vi.waitFor(() => {
      expect(fetchStats).toHaveBeenCalledWith("requests", "4w");
    });

    const buttons = wrapper.findAll("button");
    const threeMonthsButton = buttons.find((b) => b.text() === "3 months");
    expect(threeMonthsButton).toBeTruthy();
    await threeMonthsButton!.trigger("click");

    await vi.waitFor(() => {
      expect(fetchStats).toHaveBeenCalledWith("requests", "3m");
    });
  });
});
