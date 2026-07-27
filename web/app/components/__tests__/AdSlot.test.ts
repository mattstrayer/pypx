import { describe, it, expect, beforeEach, vi } from "vitest";
import { mountSuspended, mockNuxtImport } from "@nuxt/test-utils/runtime";
import AdSlot from "../AdSlot.vue";

const mockAds = {
  enabled: true,
  publisher: "pypx",
  type: "image" as "image" | "text",
  waitForFill: vi.fn(async () => true),
  reload: vi.fn(),
};

mockNuxtImport("useEthicalAds", () => () => mockAds);

describe("AdSlot", () => {
  beforeEach(() => {
    mockAds.enabled = true;
    mockAds.publisher = "pypx";
    mockAds.type = "image";
    mockAds.waitForFill = vi.fn(async () => true);
    mockAds.reload = vi.fn();
  });

  it("renders nothing at all when ads are disabled", async () => {
    mockAds.enabled = false;
    const wrapper = await mountSuspended(AdSlot);
    expect(wrapper.find("[data-ea-publisher]").exists()).toBe(false);
  });

  it("renders the placement div with publisher and type", async () => {
    const wrapper = await mountSuspended(AdSlot);
    const placement = wrapper.find("[data-ea-publisher]");
    expect(placement.exists()).toBe(true);
    expect(placement.attributes("data-ea-publisher")).toBe("pypx");
    expect(placement.attributes("data-ea-type")).toBe("image");
  });

  it("uses the configured text type", async () => {
    mockAds.type = "text";
    const wrapper = await mountSuspended(AdSlot);
    expect(wrapper.find("[data-ea-publisher]").attributes("data-ea-type")).toBe("text");
  });

  it("joins keywords with a pipe", async () => {
    const wrapper = await mountSuspended(AdSlot, {
      props: { keywords: ["django", "www/http"] },
    });
    expect(wrapper.find("[data-ea-publisher]").attributes("data-ea-keywords")).toBe(
      "django|www/http",
    );
  });

  it("omits the keywords attribute when there are none", async () => {
    const wrapper = await mountSuspended(AdSlot, { props: { keywords: [] } });
    expect(wrapper.find("[data-ea-publisher]").attributes("data-ea-keywords")).toBeUndefined();
  });

  it("shows no card chrome before a fill resolves", async () => {
    mockAds.waitForFill = vi.fn(() => new Promise<boolean>(() => {}));
    const wrapper = await mountSuspended(AdSlot);
    expect(wrapper.text()).not.toContain("Sponsored");
    expect(wrapper.find(".border-subtle").exists()).toBe(false);
  });

  it("shows no card chrome when there is no fill", async () => {
    mockAds.waitForFill = vi.fn(async () => false);
    const wrapper = await mountSuspended(AdSlot);
    await vi.waitFor(() => expect(mockAds.waitForFill).toHaveBeenCalled());
    expect(wrapper.text()).not.toContain("Sponsored");
    expect(wrapper.find(".border-subtle").exists()).toBe(false);
  });

  it("keeps the placement div visible even with no fill", async () => {
    mockAds.waitForFill = vi.fn(async () => false);
    const wrapper = await mountSuspended(AdSlot);
    await vi.waitFor(() => expect(mockAds.waitForFill).toHaveBeenCalled());
    const placement = wrapper.find("[data-ea-publisher]");
    expect(placement.exists()).toBe(true);
    expect(placement.attributes("style") ?? "").not.toContain("display: none");
  });

  it("shows the card chrome once a fill resolves", async () => {
    const wrapper = await mountSuspended(AdSlot);
    await vi.waitFor(() => expect(wrapper.text()).toContain("Sponsored"));
    expect(wrapper.find(".border-subtle").exists()).toBe(true);
  });

  it("applies sticky positioning only when asked", async () => {
    const plain = await mountSuspended(AdSlot);
    await vi.waitFor(() => expect(plain.text()).toContain("Sponsored"));
    expect(plain.find(".sticky").exists()).toBe(false);

    const sticky = await mountSuspended(AdSlot, { props: { sticky: true } });
    await vi.waitFor(() => expect(sticky.find(".sticky").exists()).toBe(true));
  });

  it("carries the adaptive-css class for dark mode", async () => {
    const wrapper = await mountSuspended(AdSlot);
    expect(wrapper.find("[data-ea-publisher]").classes()).toContain("adaptive-css");
  });
});
