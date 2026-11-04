import { describe, it, expect, afterEach } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import { nextTick } from "vue";
import DocsCommandPalette from "../DocsCommandPalette.vue";
import type { DocSymbol } from "~/types/api";
import type { DocEntry } from "~/utils/docSymbols";

function makeSymbol(
  name: string,
  kind: DocSymbol["kind"] = "function",
  module = "m",
  repeated = false,
): DocEntry {
  const path = `${module}.${name}`;
  return {
    module,
    path,
    id: path,
    repeated,
    importableAs: [],
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
    expect(wrapper.emitted("jump")![0]![0]).toBe("m.fit");
  });

  describe("constant rows", () => {
    const constants = [
      {
        name: "DEFAULT_POOLSIZE",
        kind: "constant" as const,
        module: "n.adapters",
        path: "n.adapters.DEFAULT_POOLSIZE",
        id: "n.adapters.DEFAULT_POOLSIZE",
        repeated: false,
      },
    ];

    it("shows a constant only as a search result, and jumps to its row", async () => {
      const wrapper = await mountSuspended(DocsCommandPalette, {
        props: { symbols, constants, open: true, sections: [] },
        attachTo: document.body,
      });
      expect(findInBody("palette-modal")?.textContent).not.toContain("DEFAULT_POOLSIZE");

      const input = findInBody("palette-input") as HTMLInputElement;
      input.value = "poolsize";
      input.dispatchEvent(new Event("input", { bubbles: true }));
      await nextTick();
      await nextTick();
      const row = findInBody("palette-result") as HTMLElement;
      expect(row.textContent).toContain("constant");
      expect(row.textContent).toContain("DEFAULT_POOLSIZE");
      row.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      await nextTick();
      expect(wrapper.emitted("jump")![0]![0]).toBe("n.adapters.DEFAULT_POOLSIZE");
    });
  });

  it("jumps to a section by its target", async () => {
    const wrapper = await mountSuspended(DocsCommandPalette, {
      props: {
        symbols,
        open: true,
        sections: [
          { label: "Re-exports", kind: "re-exports", count: 3, firstSymbol: "re-exports" },
        ],
      },
      attachTo: document.body,
    });
    const section = findInBody("palette-section") as HTMLElement;
    expect(section.textContent).toContain("Re-exports");
    section.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await nextTick();
    expect(wrapper.emitted("jump")![0]![0]).toBe("re-exports");
  });

  describe("alias rows", () => {
    const aliases = [
      { name: "fit_alias", module: "n.sub", path: "n.sub.fit_alias", targetId: "m.fit" },
    ];

    async function search(query: string) {
      const wrapper = await mountSuspended(DocsCommandPalette, {
        props: { symbols, aliases, open: true, sections: [] },
        attachTo: document.body,
      });
      const input = findInBody("palette-input") as HTMLInputElement;
      input.value = query;
      input.dispatchEvent(new Event("input", { bubbles: true }));
      await nextTick();
      await nextTick();
      return wrapper;
    }

    it("are not listed before a search", async () => {
      await mountSuspended(DocsCommandPalette, {
        props: { symbols, aliases, open: true, sections: [] },
        attachTo: document.body,
      });
      expect(findInBody("palette-alias")).toBeNull();
    });

    it("show the alias name and module, and jump to the target card", async () => {
      const wrapper = await search("fit_alias");
      const row = findInBody("palette-alias") as HTMLElement;
      expect(row.textContent).toContain("fit_alias");
      expect(row.textContent).toContain("n.sub");
      row.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      await nextTick();
      expect(wrapper.emitted("jump")![0]![0]).toBe("m.fit");
    });

    it("match on the dotted path", async () => {
      await search("n.sub.fit");
      expect(findInBody("palette-alias")).not.toBeNull();
    });
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

  it("finds a symbol by its module path and shows the module of a repeated name", async () => {
    const wrapper = await mountSuspended(DocsCommandPalette, {
      props: {
        symbols: [
          makeSymbol("Request", "class", "werkzeug.sansio.request", true),
          makeSymbol("Request", "class", "werkzeug.wrappers.request", true),
        ],
        open: true,
        sections: [],
      },
      attachTo: document.body,
    });
    const input = document.body.querySelector("input") as HTMLInputElement;
    input.value = "wrappers.request.Request";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    await nextTick();
    const first = findInBody("palette-result") as HTMLElement;
    expect(findInBody("palette-result-module")?.textContent).toContain("werkzeug.wrappers.request");
    first.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await nextTick();
    expect(wrapper.emitted("jump")![0]![0]).toBe("werkzeug.wrappers.request.Request");
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
