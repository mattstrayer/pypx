import { describe, it, expect, vi, afterEach } from "vitest";
import { mountSuspended, mockNuxtImport } from "@nuxt/test-utils/runtime";
import { nextTick } from "vue";
import CommandPalette from "../CommandPalette.vue";

// Stub the search API — the palette never types in these tests, but mounting
// wires up useSearchTypeahead → useApi.
const mockSearchPackages = vi.fn().mockResolvedValue([]);
mockNuxtImport("useApi", () => () => ({ searchPackages: mockSearchPackages }));

let wrapper: Awaited<ReturnType<typeof mountSuspended>> | null = null;

async function mountPalette() {
  wrapper = await mountSuspended(CommandPalette, { attachTo: document.body });
  return wrapper;
}

function findDialog(): HTMLElement | null {
  return document.body.querySelector('[role="dialog"]');
}

async function openPalette() {
  window.dispatchEvent(new KeyboardEvent("keydown", { key: "k", metaKey: true }));
  await nextTick();
  await nextTick();
}

describe("CommandPalette", () => {
  afterEach(() => {
    // Unmount so the global keydown listener is removed and does not leak into
    // the next test (a stale listener would steal focus on ⌘K).
    wrapper?.unmount();
    wrapper = null;
    document.body.innerHTML = "";
  });

  it("opens on ⌘K and the input has combobox ARIA wiring", async () => {
    await mountPalette();

    await openPalette();

    const dialog = findDialog();
    expect(dialog).not.toBeNull();

    const input = dialog!.querySelector("input") as HTMLInputElement;
    expect(input.getAttribute("role")).toBe("combobox");
    expect(input.getAttribute("aria-controls")).toBe("palette-search-listbox");
    expect(input.getAttribute("aria-autocomplete")).toBe("list");
  });

  it("restores focus to the previously focused element on Escape close", async () => {
    await mountPalette();

    // A dummy trigger element that had focus before opening the palette.
    const trigger = document.createElement("button");
    trigger.textContent = "trigger";
    document.body.appendChild(trigger);
    trigger.focus();
    expect(document.activeElement).toBe(trigger);

    await openPalette();
    const dialog = findDialog();
    expect(dialog).not.toBeNull();

    const input = dialog!.querySelector("input") as HTMLInputElement;
    input.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
    await nextTick();

    expect(findDialog()).toBeNull();
    expect(document.activeElement).toBe(trigger);
  });
});
