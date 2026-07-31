import { describe, it, expect } from "vitest";
import { mountSuspended, mockNuxtImport } from "@nuxt/test-utils/runtime";
import { reactive } from "vue";
import DefaultLayout from "../default.vue";

const routeMock = reactive({
  path: "/packages/requests",
  fullPath: "/packages/requests",
  query: {},
});

mockNuxtImport("useRoute", () => () => routeMock);

describe("default layout agent discovery footer link", () => {
  it("renders an API for agents link pointing at /llms.txt", async () => {
    const wrapper = await mountSuspended(DefaultLayout);

    const link = wrapper.find('a[href="/llms.txt"]');
    expect(link.exists()).toBe(true);
    expect(link.text()).toBe("API for agents");
  });
});

// Nick Launches pulls a free listing if the backlink stops resolving, so this
// is a distribution dependency rather than decoration. The assertions below
// pin the parts their check and our referral attribution actually rely on.
describe("default layout Nick Launches badge", () => {
  it("links back to Nick Launches", async () => {
    const wrapper = await mountSuspended(DefaultLayout);

    const badge = wrapper.find('a[href^="https://nicklaunches.com"]');
    expect(badge.exists()).toBe(true);
  });

  it("keeps the Referer header intact so referral traffic is attributed", async () => {
    const wrapper = await mountSuspended(DefaultLayout);

    const rel = wrapper.find('a[href^="https://nicklaunches.com"]').attributes("rel");
    expect(rel).toContain("noopener");
    expect(rel).not.toContain("noreferrer");
  });

  it("ships a badge for each theme, with dimensions set to avoid layout shift", async () => {
    const wrapper = await mountSuspended(DefaultLayout);

    const imgs = wrapper.findAll('a[href^="https://nicklaunches.com"] img');
    expect(imgs).toHaveLength(2);

    for (const img of imgs) {
      expect(img.attributes("width")).toBe("240");
      expect(img.attributes("height")).toBe("56");
      expect(img.attributes("loading")).toBe("lazy");
      // The anchor carries the accessible name; naming the images too would
      // announce the link twice.
      expect(img.attributes("alt")).toBe("");
    }

    expect(imgs[0]!.classes()).toContain("dark:hidden");
    expect(imgs[1]!.classes()).toContain("dark:block");
  });
});
