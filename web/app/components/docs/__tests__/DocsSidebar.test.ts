import { describe, it, expect, vi } from "vitest";
import { defineComponent, h } from "vue";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import DocsSidebar from "../DocsSidebar.vue";
import type { DocSymbol } from "~/types/api";
import type { DocEntry } from "~/utils/docSymbols";

vi.mock("vue-virtual-scroller", () => ({
  DynamicScroller: defineComponent({
    props: ["items", "minItemSize", "keyField"],
    setup(props, { slots }) {
      return () =>
        h(
          "div",
          props.items.flatMap(
            (item: unknown, index: number) => slots.default?.({ item, index, active: true }) ?? [],
          ),
        );
    },
  }),
  DynamicScrollerItem: defineComponent({
    props: ["item", "active"],
    setup(_props, { slots }) {
      return () => h("div", slots.default?.() ?? []);
    },
  }),
}));

function makeSymbol(
  name: string,
  kind: DocSymbol["kind"] = "function",
  module = "m",
  repeated = false,
): DocEntry {
  const path = `${module}.${name}`;
  return {
    name,
    kind,
    signature: "",
    docstring: "",
    parameters: [],
    raises: [],
    module,
    path,
    id: path,
    repeated,
  };
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

  it("shows the module only on names that repeat, and selects by id", async () => {
    const wrapper = await mountSuspended(DocsSidebar, {
      props: {
        functions: [],
        classes: [
          makeSymbol("Redis", "class", "redis.asyncio.client", true),
          makeSymbol("Redis", "class", "redis.client", true),
          makeSymbol("Lock", "class", "redis.lock"),
        ],
        exceptions: [],
        activeSymbol: "redis.client.Redis",
      },
    });
    const rows = wrapper.findAll("[data-testid='symbol-row']");
    expect(rows).toHaveLength(3);
    expect(rows[0]!.find("[data-testid='symbol-row-module']").text()).toBe("redis.asyncio.client");
    expect(rows[2]!.find("[data-testid='symbol-row-module']").exists()).toBe(false);
    expect(rows[0]!.attributes("aria-current")).toBeUndefined();
    expect(rows[1]!.attributes("aria-current")).toBe("location");
    await rows[1]!.trigger("click");
    expect(wrapper.emitted("select")![0]![0]).toBe("redis.client.Redis");
  });

  it("emits select with symbol name when a row is clicked", async () => {
    const wrapper = await mountSuspended(DocsSidebar, {
      props: { functions, classes, exceptions: [], activeSymbol: null },
    });
    await wrapper.find("[data-testid='symbol-row']").trigger("click");
    expect(wrapper.emitted("select")).toBeTruthy();
    expect(typeof wrapper.emitted("select")![0]![0]).toBe("string");
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
