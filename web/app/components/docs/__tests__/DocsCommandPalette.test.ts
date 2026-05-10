import { describe, it, expect, afterEach } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import { nextTick } from "vue";
import DocsCommandPalette from "../DocsCommandPalette.vue";
import type { DocSymbol } from "~/types/api";

function makeSymbol(name: string, kind: DocSymbol["kind"] = "function"): DocSymbol {
  return {
    name,
    kind,
    signature: `def ${name}()`,
    docstring: "",
    parameters: [],
    raises: [],
  };
}

const symbols = [
  makeSymbol("fit"),
  makeSymbol("predict"),
  makeSymbol("fit_transform"),
  makeSymbol("Pipeline", "class"),
  makeSymbol("GridSearchCV", "class"),
];

// Helper to query teleported content in document.body
function findInBody(testId: string): Element | null {
  return document.body.querySelector(`[data-testid='${testId}']`);
}

describe("DocsCommandPalette", () => {
  afterEach(() => {
    // Clean up any attached wrappers
    document.body.innerHTML = "";
  });

  it("is not visible when open is false", async () => {
    const wrapper = await mountSuspended(DocsCommandPalette, {
      props: { symbols, open: false, sections: [] },
    });
    expect(wrapper.find("[data-testid='palette-modal']").exists()).toBe(false);
  });

  it("is visible when open is true", async () => {
    await mountSuspended(DocsCommandPalette, {
      props: { symbols, open: true, sections: [] },
      attachTo: document.body,
    });
    expect(findInBody("palette-modal")).not.toBeNull();
  });

  it("shows all symbols grouped when query is empty", async () => {
    await mountSuspended(DocsCommandPalette, {
      props: { symbols, open: true, sections: [] },
      attachTo: document.body,
    });
    const modal = findInBody("palette-modal");
    expect(modal?.textContent).toContain("fit");
    expect(modal?.textContent).toContain("Pipeline");
  });

  it("filters symbols by query", async () => {
    await mountSuspended(DocsCommandPalette, {
      props: { symbols, open: true, sections: [] },
      attachTo: document.body,
    });
    const input = findInBody("palette-input") as HTMLInputElement;
    // Simulate v-model input
    input.value = "pipeline";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    await nextTick();
    await nextTick();
    const modal = findInBody("palette-modal");
    expect(modal?.textContent).toContain("Pipeline");
    expect(modal?.textContent).not.toContain("predict");
  });

  it("emits jump when a result is clicked", async () => {
    const wrapper = await mountSuspended(DocsCommandPalette, {
      props: { symbols, open: true, sections: [] },
      attachTo: document.body,
    });
    const firstResult = findInBody("palette-result") as HTMLElement;
    firstResult.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await nextTick();
    expect(wrapper.emitted("jump")).toBeTruthy();
    expect(wrapper.emitted("jump")![0]!).toHaveLength(1);
    expect(wrapper.emitted("jump")![0]![0]).toBe("fit");
  });

  it("emits close when Escape is pressed", async () => {
    const wrapper = await mountSuspended(DocsCommandPalette, {
      props: { symbols, open: true, sections: [] },
      attachTo: document.body,
    });
    const input = findInBody("palette-input") as HTMLInputElement;
    input.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
    await nextTick();
    expect(wrapper.emitted("close")).toBeTruthy();
  });

  it("emits close when backdrop is clicked", async () => {
    const wrapper = await mountSuspended(DocsCommandPalette, {
      props: { symbols, open: true, sections: [] },
      attachTo: document.body,
    });
    const backdrop = findInBody("palette-backdrop") as HTMLElement;
    backdrop.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await nextTick();
    expect(wrapper.emitted("close")).toBeTruthy();
  });

  it("shows section items when open", async () => {
    const sections = [
      { label: "Functions", kind: "functions", count: 3, firstSymbol: "fit" },
      { label: "Classes", kind: "classes", count: 2, firstSymbol: "Pipeline" },
    ];
    await mountSuspended(DocsCommandPalette, {
      props: { symbols, open: true, sections },
      attachTo: document.body,
    });
    const sectionEls = document.body.querySelectorAll("[data-testid='palette-section']");
    expect(sectionEls.length).toBe(2);
    expect(document.body.textContent).toContain("Functions");
    expect(document.body.textContent).toContain("Classes");
  });

  it("emits jump with firstSymbol when a section is clicked", async () => {
    const sections = [{ label: "Functions", kind: "functions", count: 3, firstSymbol: "fit" }];
    const wrapper = await mountSuspended(DocsCommandPalette, {
      props: { symbols, open: true, sections },
      attachTo: document.body,
    });
    const sectionEl = document.body.querySelector("[data-testid='palette-section']");
    sectionEl?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await nextTick();
    expect(wrapper.emitted("jump")).toBeTruthy();
    expect(wrapper.emitted("jump")![0]![0]).toBe("fit");
  });
});
