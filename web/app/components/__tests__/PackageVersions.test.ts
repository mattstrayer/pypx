import { describe, it, expect, vi } from "vitest";
import { mountSuspended, mockNuxtImport } from "@nuxt/test-utils/runtime";
import PackageVersions from "../PackageVersions.vue";

const fetchVersions = vi.fn().mockResolvedValue([]);
const fetchChangelog = vi.fn().mockResolvedValue({ entries: [] });

mockNuxtImport("useApi", () => () => ({ fetchVersions, fetchChangelog }));

describe("PackageVersions", () => {
  it("refetches versions and changelog when the name prop changes", async () => {
    const wrapper = await mountSuspended(PackageVersions, { props: { name: "left" } });

    await vi.waitFor(() => {
      expect(fetchVersions).toHaveBeenCalledWith("left");
    });

    await wrapper.setProps({ name: "right" });

    await vi.waitFor(() => {
      expect(fetchVersions).toHaveBeenCalledWith("right");
    });
  });
});
