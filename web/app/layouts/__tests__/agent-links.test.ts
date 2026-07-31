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
  it("links to the pypx listing, not the Nick Launches homepage", async () => {
    const wrapper = await mountSuspended(DefaultLayout);

    const href = wrapper.find('a[href^="https://nicklaunches.com"]').attributes("href");
    expect(href).toContain("/products/pypx/");
  });

  it("keeps the UTM params intact so the referral is attributed to the badge", async () => {
    const wrapper = await mountSuspended(DefaultLayout);

    const href = wrapper.find('a[href^="https://nicklaunches.com"]').attributes("href")!;
    // A literal `&amp;` here would break their attribution silently.
    expect(href).not.toContain("&amp;");
    const params = new URL(href).searchParams;
    expect(params.get("utm_source")).toBe("pypx.app");
    expect(params.get("utm_medium")).toBe("badge");
    expect(params.get("utm_campaign")).toBe("featured");
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
      // Only one variant is ever in the a11y tree, so both carry the real alt
      // and the visible one names the link.
      expect(img.attributes("alt")).toBe("pypx on Nick Launches");
    }

    expect(imgs[0]!.classes()).toContain("dark:hidden");
    expect(imgs[1]!.classes()).toContain("dark:block");
  });
});
