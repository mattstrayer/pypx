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

  it("exposes sym-{name} id for scroll targeting", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol({ name: "predict" }), isActive: false },
    });
    expect(wrapper.find("#sym-predict").exists()).toBe(true);
  });
});
