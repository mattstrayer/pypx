import { describe, it, expect } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import DocsSymbolCard from "../DocsSymbolCard.vue";
import type { DocSymbol } from "~/types/api";

function makeSymbol(overrides: Partial<DocSymbol> = {}): DocSymbol {
  return {
    name: "fit",
    kind: "function",
    signature: "def fit(X, y=None)",
    docstring: "Fit the model.",
    parameters: [
      { name: "X", type: "array", description: "Training data.", kind: "positional_or_keyword" },
    ],
    returns: { type: "self", description: "The fitted estimator." },
    raises: [],
    ...overrides,
  };
}

describe("DocsSymbolCard", () => {
  it("renders symbol name", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol(), isActive: false },
    });
    expect(wrapper.text()).toContain("fit");
  });

  it("renders function kind badge", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol(), isActive: false },
    });
    expect(wrapper.text()).toContain("function");
  });

  it("renders class kind badge", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol({ name: "Pipeline", kind: "class" }), isActive: false },
    });
    expect(wrapper.text()).toContain("class");
  });

  it("shows expand button for classes with methods", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: {
        symbol: makeSymbol({
          name: "Pipeline",
          kind: "class",
          methods: [makeSymbol({ name: "fit_transform" })],
        }),
        isActive: false,
      },
    });
    expect(wrapper.text()).toMatch(/methods/i);
  });

  it("does not show methods section for functions", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol(), isActive: false },
    });
    expect(wrapper.text()).not.toMatch(/methods/i);
  });

  it("shows methods for exceptions", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: {
        symbol: makeSymbol({
          name: "HTTPError",
          kind: "exception",
          methods: [makeSymbol({ name: "explain" })],
        }),
        isActive: false,
      },
    });
    expect(wrapper.text()).toMatch(/methods/i);
    await wrapper.find("button").trigger("click");
    expect(wrapper.text()).toContain("explain");
  });

  it.each(["class", "exception"] as const)("shows attributes on a %s", async (kind) => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: {
        symbol: makeSymbol({
          name: "Thing",
          kind,
          attributes: [
            {
              name: "timeout",
              type: "float",
              default: "30.0",
              description: "Seconds to wait.",
              is_property: false,
              writable: false,
            },
          ],
        }),
        isActive: false,
      },
    });
    const text = wrapper.text();
    expect(text).toContain("Attributes");
    expect(text).toContain("timeout");
    expect(text).toContain("float");
    expect(text).toContain("= 30.0");
    expect(text).toContain("Seconds to wait.");
    expect(text).not.toMatch(/read-only/);
    expect(text).not.toContain("property");
  });

  it("labels a read-only property", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: {
        symbol: makeSymbol({
          name: "Thing",
          kind: "class",
          attributes: [{ name: "size", description: "", is_property: true, writable: false }],
        }),
        isActive: false,
      },
    });
    expect(wrapper.text()).toContain("property");
    expect(wrapper.text()).toContain("read-only");
  });

  it("does not label a writable property read-only", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: {
        symbol: makeSymbol({
          name: "Thing",
          kind: "class",
          attributes: [{ name: "size", description: "", is_property: true, writable: true }],
        }),
        isActive: false,
      },
    });
    expect(wrapper.text()).toContain("property");
    expect(wrapper.text()).not.toContain("read-only");
  });

  it("hides the attributes section when a class has none", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol({ kind: "class", attributes: [] }), isActive: false },
    });
    expect(wrapper.text()).not.toContain("Attributes");
  });

  it("exposes sym-{name} id for scroll targeting", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol({ name: "predict" }), isActive: false },
    });
    expect(wrapper.find("#sym-predict").exists()).toBe(true);
  });
});
