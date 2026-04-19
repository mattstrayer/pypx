import { describe, it, expect, vi } from "vitest";
import { defineComponent, h } from "vue";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import DocsSidebar from "../DocsSidebar.vue";
import type { DocSymbol } from "~/types/api";

vi.mock("vue-virtual-scroller", () => ({
  DynamicScroller: defineComponent({
    props: ["items", "minItemSize", "keyField"],
    setup(props, { slots }) {
      return () =>
        h(
          "div",
          props.items.map(
            (item: unknown, index: number) =>
              slots.default?.({ item, index, active: true }) ?? null,
          ),
        );
    },
  }),
  DynamicScrollerItem: defineComponent({
    props: ["item", "active"],
    setup(_props, { slots }) {
      return () => h("div", slots.default?.() ?? null);
    },
  }),
}));

function makeSymbol(name: string, kind: DocSymbol["kind"] = "function"): DocSymbol {
  return { name, kind, signature: "", docstring: "", parameters: [], returns: null, raises: [] };
}

const functions = [makeSymbol("fit"), makeSymbol("predict"), makeSymbol("transform")];
const classes = [makeSymbol("Pipeline", "class"), makeSymbol("GridSearchCV", "class")];

describe("DocsSidebar", () => {
  it("renders Functions section header with count", async () => {
    const wrapper = await mountSuspended(DocsSidebar, {
      props: { functions, classes, exceptions: [], activeSymbol: null },
    });
    expect(wrapper.text()).toContain("Functions");
    expect(wrapper.text()).toContain("3");
  });

  it("renders Classes section header with count", async () => {
    const wrapper = await mountSuspended(DocsSidebar, {
      props: { functions, classes, exceptions: [], activeSymbol: null },
    });
    expect(wrapper.text()).toContain("Classes");
    expect(wrapper.text()).toContain("2");
  });

  it("does not render empty sections", async () => {
    const wrapper = await mountSuspended(DocsSidebar, {
      props: { functions, classes, exceptions: [], activeSymbol: null },
    });
    expect(wrapper.text()).not.toContain("Exceptions");
  });

  it("emits select with symbol name when a row is clicked", async () => {
    const wrapper = await mountSuspended(DocsSidebar, {
      props: { functions, classes, exceptions: [], activeSymbol: null },
    });
    await wrapper.find("[data-testid='symbol-row']").trigger("click");
    expect(wrapper.emitted("select")).toBeTruthy();
    expect(typeof wrapper.emitted("select")![0][0]).toBe("string");
  });

  it("collapses functions section when header is clicked", async () => {
    const wrapper = await mountSuspended(DocsSidebar, {
      props: { functions, classes: [], exceptions: [], activeSymbol: null },
    });
    // Before collapse: symbol rows exist
    expect(wrapper.findAll("[data-testid='symbol-row']").length).toBeGreaterThan(0);
    // Click section header to collapse
    await wrapper.find("[data-testid='section-header']").trigger("click");
    // After collapse: no symbol rows visible
    expect(wrapper.findAll("[data-testid='symbol-row']").length).toBe(0);
  });

  it("shows collapse indicator on section headers", async () => {
    const wrapper = await mountSuspended(DocsSidebar, {
      props: { functions, classes, exceptions: [], activeSymbol: null },
    });
    expect(wrapper.find("[data-testid='section-header']").find("svg").exists()).toBe(true);
  });
});
