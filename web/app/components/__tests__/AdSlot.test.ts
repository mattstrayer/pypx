import { describe, it, expect, beforeEach, vi } from "vitest";
import { mountSuspended, mockNuxtImport } from "@nuxt/test-utils/runtime";
import AdSlot from "../AdSlot.vue";

const mockAds = {
  enabled: true,
  publisher: "pypx",
  type: "image" as "image" | "text",
  reload: vi.fn(),
};

mockNuxtImport("useEthicalAds", () => () => mockAds);

function appendChild(el: Element): HTMLElement {
  const child = document.createElement("div");
  el.appendChild(child);
  return child;
}

function setVendorClient(reload: () => void): void {
  (window as unknown as Record<string, unknown>).ethicalads = { reload };
}

describe("AdSlot", () => {
  beforeEach(() => {
    mockAds.enabled = true;
    mockAds.publisher = "pypx";
    mockAds.type = "image";
    mockAds.reload = vi.fn();
    delete (window as unknown as Record<string, unknown>).ethicalads;
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

  it("shows no card chrome when the placement div is empty", async () => {
    const wrapper = await mountSuspended(AdSlot);
    expect(wrapper.text()).not.toContain("Sponsored");
    expect(wrapper.find(".border-subtle").exists()).toBe(false);
    const placement = wrapper.find("[data-ea-publisher]");
    expect(placement.exists()).toBe(true);
    expect(placement.attributes("style") ?? "").not.toContain("display: none");
  });

  it("shows the card chrome once the vendor client injects a child", async () => {
    const wrapper = await mountSuspended(AdSlot);
    const placement = wrapper.find("[data-ea-publisher]").element;
    appendChild(placement);

    await vi.waitFor(() => expect(wrapper.text()).toContain("Sponsored"));
    expect(wrapper.find(".border-subtle").exists()).toBe(true);
  });

  it("removes the card chrome again once the injected child is removed", async () => {
    const wrapper = await mountSuspended(AdSlot);
    const placement = wrapper.find("[data-ea-publisher]").element;
    const child = appendChild(placement);
    await vi.waitFor(() => expect(wrapper.text()).toContain("Sponsored"));

    placement.removeChild(child);
    await vi.waitFor(() => expect(wrapper.text()).not.toContain("Sponsored"));
    expect(wrapper.find(".border-subtle").exists()).toBe(false);
    expect(wrapper.find("[data-ea-publisher]").exists()).toBe(true);
  });

  it("applies sticky positioning only when asked", async () => {
    const plain = await mountSuspended(AdSlot);
    appendChild(plain.find("[data-ea-publisher]").element);
    await vi.waitFor(() => expect(plain.text()).toContain("Sponsored"));
    expect(plain.find(".sticky").exists()).toBe(false);

    const sticky = await mountSuspended(AdSlot, { props: { sticky: true } });
    appendChild(sticky.find("[data-ea-publisher]").element);
    await vi.waitFor(() => expect(sticky.find(".sticky").exists()).toBe(true));
  });

  it("carries the adaptive-css class for dark mode", async () => {
    const wrapper = await mountSuspended(AdSlot);
    expect(wrapper.find("[data-ea-publisher]").classes()).toContain("adaptive-css");
  });

  it("calls reload and resets filled when keywords change", async () => {
    const wrapper = await mountSuspended(AdSlot, { props: { keywords: ["django"] } });
    appendChild(wrapper.find("[data-ea-publisher]").element);
    await vi.waitFor(() => expect(wrapper.text()).toContain("Sponsored"));
    mockAds.reload.mockClear();

    await wrapper.setProps({ keywords: ["flask"] });

    expect(mockAds.reload).toHaveBeenCalledOnce();
    expect(wrapper.text()).not.toContain("Sponsored");
  });

  // The vendor's reload() synchronously re-reads the keywords off the DOM
  // attribute, so it is not enough that reload was called — it must be called
  // AFTER Vue has patched the new keywords onto the placement div. Under the
  // default `flush: "pre"` the watcher runs before the patch and this captures
  // the PREVIOUS package's keywords.
  it("reloads only after the new keywords are on the placement div", async () => {
    const seen: (string | null | undefined)[] = [];
    mockAds.reload = vi.fn(() => {
      seen.push(document.querySelector("[data-ea-publisher]")?.getAttribute("data-ea-keywords"));
    });

    const wrapper = await mountSuspended(AdSlot, {
      props: { keywords: ["www/http"] },
      attachTo: document.body,
    });
    expect(seen).toEqual(["www/http"]);
    seen.length = 0;

    await wrapper.setProps({ keywords: ["django", "www/http"] });

    expect(seen).toEqual(["django|www/http"]);
    wrapper.unmount();
  });

  // Crossing the 1024px breakpoint swaps one AdSlot for the other while the
  // app-scoped script tag stays put and never re-executes. Without this the new
  // slot is never discovered and stays silently empty forever.
  it("reloads on mount so a slot mounted after the client script is discovered", async () => {
    const vendorReload = vi.fn();
    setVendorClient(vendorReload);
    // Mirror the composable: reload() delegates to the vendor global when present.
    mockAds.reload = vi.fn(() => {
      (window as unknown as { ethicalads?: { reload: () => void } }).ethicalads?.reload();
    });

    await mountSuspended(AdSlot, { props: { keywords: ["django"] } });

    expect(mockAds.reload).toHaveBeenCalledOnce();
    expect(vendorReload).toHaveBeenCalledOnce();
  });

  it("mounts without throwing when the vendor client has not loaded yet", async () => {
    expect((window as unknown as Record<string, unknown>).ethicalads).toBeUndefined();

    const wrapper = await mountSuspended(AdSlot, { props: { keywords: ["django"] } });

    expect(wrapper.find("[data-ea-publisher]").exists()).toBe(true);
    expect(mockAds.reload).toHaveBeenCalledOnce();
  });

  it("disconnects the observer on unmount", async () => {
    const disconnectSpy = vi.spyOn(MutationObserver.prototype, "disconnect");
    const wrapper = await mountSuspended(AdSlot);
    const placement = wrapper.find("[data-ea-publisher]").element;

    wrapper.unmount();
    expect(disconnectSpy).toHaveBeenCalled();

    // Mutating the now-detached node must not throw and must not resurrect
    // any reactive state on the unmounted component.
    expect(() => appendChild(placement)).not.toThrow();
    disconnectSpy.mockRestore();
  });
});
