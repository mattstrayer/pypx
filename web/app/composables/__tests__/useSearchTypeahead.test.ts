import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { defineComponent, h } from "vue";
import { mountSuspended, mockNuxtImport } from "@nuxt/test-utils/runtime";
import { flushPromises } from "@vue/test-utils";
import { useSearchTypeahead } from "../useSearchTypeahead";
import type { SearchResult } from "~/types/api";

// ---- Stubs ----

const mockSearchPackages = vi.fn();
mockNuxtImport("useApi", () => () => ({ searchPackages: mockSearchPackages }));

// ---- Helpers ----

function makeResult(name: string, downloads = 1000): SearchResult {
  return { name, summary: "", downloads };
}

/**
 * Mounts a harness that captures the raw composable return value (refs intact).
 * The component `expose`s the composable result so we can access refs via
 * wrapper.vm.$el or wrapper.getCurrentComponent().exposed.
 */
async function mountTypeahead() {
  let composable!: ReturnType<typeof useSearchTypeahead>;

  const Harness = defineComponent({
    setup() {
      composable = useSearchTypeahead();
      return composable;
    },
    render: () => h("div"),
  });

  await mountSuspended(Harness);
  return composable;
}

// ---- Tests ----

describe("useSearchTypeahead", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockSearchPackages.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("typing a query sets isLoading=true and isOpen=true; after debounce results populate and isLoading=false", async () => {
    const stubResults = [makeResult("requests"), makeResult("httpx")];
    mockSearchPackages.mockResolvedValue(stubResults);

    const c = await mountTypeahead();

    c.query.value = "req";
    await flushPromises();

    expect(c.isLoading.value).toBe(true);
    expect(c.isOpen.value).toBe(true);

    vi.advanceTimersByTime(150);
    await flushPromises();

    expect(c.results.value).toEqual(stubResults);
    expect(c.isLoading.value).toBe(false);
    expect(mockSearchPackages).toHaveBeenCalledWith("req", 5);
  });

  it("clearing the query closes the dropdown and empties results without fetching", async () => {
    const stubResults = [makeResult("requests")];
    mockSearchPackages.mockResolvedValue(stubResults);

    const c = await mountTypeahead();

    // First populate
    c.query.value = "req";
    await flushPromises();
    vi.advanceTimersByTime(150);
    await flushPromises();
    expect(c.results.value).toEqual(stubResults);

    // Then clear
    mockSearchPackages.mockClear();
    c.query.value = "";
    await flushPromises();
    vi.advanceTimersByTime(150);
    await flushPromises();

    expect(c.isOpen.value).toBe(false);
    expect(c.results.value).toEqual([]);
    // searchPackages must NOT be called for empty query
    expect(mockSearchPackages).not.toHaveBeenCalled();
  });

  it("rejection from searchPackages yields results=[] and no throw (swallow behavior)", async () => {
    mockSearchPackages.mockRejectedValue(new Error("network error"));

    const c = await mountTypeahead();

    c.query.value = "err";
    await flushPromises();
    vi.advanceTimersByTime(150);
    await flushPromises();

    expect(c.results.value).toEqual([]);
    expect(c.isLoading.value).toBe(false);
  });

  describe("keyboard navigation", () => {
    async function setupWithResults(count: number) {
      const stubResults = Array.from({ length: count }, (_, i) => makeResult(`pkg${i}`));
      mockSearchPackages.mockResolvedValue(stubResults);

      const c = await mountTypeahead();

      c.query.value = "pkg";
      await flushPromises();
      vi.advanceTimersByTime(150);
      await flushPromises();

      return { c, stubResults };
    }

    it("ArrowDown increments selectedIndex within bounds", async () => {
      const { c } = await setupWithResults(3);

      expect(c.selectedIndex.value).toBe(-1);
      c.onKeydown(new KeyboardEvent("keydown", { key: "ArrowDown" }));
      expect(c.selectedIndex.value).toBe(0);
      c.onKeydown(new KeyboardEvent("keydown", { key: "ArrowDown" }));
      expect(c.selectedIndex.value).toBe(1);
      c.onKeydown(new KeyboardEvent("keydown", { key: "ArrowDown" }));
      expect(c.selectedIndex.value).toBe(2);
      // Capped at last index (2 for 3 items)
      c.onKeydown(new KeyboardEvent("keydown", { key: "ArrowDown" }));
      expect(c.selectedIndex.value).toBe(2);
    });

    it("ArrowUp decrements selectedIndex with floor of -1", async () => {
      const { c } = await setupWithResults(3);

      c.onKeydown(new KeyboardEvent("keydown", { key: "ArrowDown" }));
      c.onKeydown(new KeyboardEvent("keydown", { key: "ArrowDown" }));
      expect(c.selectedIndex.value).toBe(1);

      c.onKeydown(new KeyboardEvent("keydown", { key: "ArrowUp" }));
      expect(c.selectedIndex.value).toBe(0);
      c.onKeydown(new KeyboardEvent("keydown", { key: "ArrowUp" }));
      expect(c.selectedIndex.value).toBe(-1);
      // Floor at -1
      c.onKeydown(new KeyboardEvent("keydown", { key: "ArrowUp" }));
      expect(c.selectedIndex.value).toBe(-1);
    });

    it("Enter with a selected item calls router.push with the package route", async () => {
      const { c } = await setupWithResults(3);

      const router = useRouter();
      const pushSpy = vi.spyOn(router, "push").mockResolvedValue(undefined);

      c.onKeydown(new KeyboardEvent("keydown", { key: "ArrowDown" }));
      c.onKeydown(new KeyboardEvent("keydown", { key: "Enter" }));

      expect(pushSpy).toHaveBeenCalledWith("/packages/pkg0");
      pushSpy.mockRestore();
    });

    it("Enter with no selection (selectedIndex=-1) does NOT navigate", async () => {
      const { c } = await setupWithResults(3);

      const router = useRouter();
      const pushSpy = vi.spyOn(router, "push").mockResolvedValue(undefined);

      expect(c.selectedIndex.value).toBe(-1);
      c.onKeydown(new KeyboardEvent("keydown", { key: "Enter" }));

      expect(pushSpy).not.toHaveBeenCalled();
      pushSpy.mockRestore();
    });

    it("Escape closes the dropdown and resets selectedIndex", async () => {
      const { c } = await setupWithResults(2);

      expect(c.isOpen.value).toBe(true);
      c.onKeydown(new KeyboardEvent("keydown", { key: "Escape" }));
      expect(c.isOpen.value).toBe(false);
      expect(c.selectedIndex.value).toBe(-1);
    });
  });
});
