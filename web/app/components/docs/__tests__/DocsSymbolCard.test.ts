import { describe, it, expect } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import DocsSymbolCard from "../DocsSymbolCard.vue";
import type { DocEntry } from "~/utils/docSymbols";

function makeSymbol(overrides: Partial<DocEntry> = {}): DocEntry {
  return {
    module: "sklearn.base",
    path: "sklearn.base.fit",
    id: "sklearn.base.fit",
    repeated: false,
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
  it("shows the module and uses the id as its anchor", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol(), isActive: false },
    });
    expect(wrapper.find("[data-testid='symbol-module']").text()).toBe("sklearn.base");
    expect(wrapper.find("[id='sym-sklearn.base.fit']").exists()).toBe(true);
  });

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

  it("renders inline markup in structured descriptions, and escapes them", async () => {
    const markup = "Uses :attr:`~a.limit` and **care**. <img src=x>";
    const entry = {
      parameters: [{ name: "x", description: markup, kind: "positional_or_keyword" }],
      returns: { type: "int", description: markup },
      raises: [{ type: "ValueError", description: markup }],
    };
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: {
        symbol: makeSymbol({ kind: "class", ...entry, methods: [makeSymbol(entry)] }),
        isActive: false,
      },
    });
    await wrapper.find("button").trigger("click");
    const html = wrapper.html();
    // Three on the card and three on the method.
    expect(html.match(/<code>limit<\/code>/g)).toHaveLength(6);
    expect(html.match(/<strong>care<\/strong>/g)).toHaveLength(6);
    expect(html).not.toContain(":attr:");
    expect(html).not.toContain("<img");
  });

  it("labels classmethods and staticmethods on method rows", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: {
        symbol: makeSymbol({
          name: "Pipeline",
          kind: "class",
          methods: [
            makeSymbol({ name: "from_steps", method_kind: "classmethod" }),
            makeSymbol({ name: "helper", method_kind: "staticmethod" }),
            makeSymbol({ name: "fit" }),
          ],
        }),
        isActive: false,
      },
    });
    await wrapper.find("button").trigger("click");
    expect(wrapper.text()).toContain("from_stepsmethodclassmethod");
    expect(wrapper.text()).toContain("helpermethodstaticmethod");
    expect(wrapper.text()).toMatch(/fitmethod(?!classmethod|staticmethod)/);
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

  it("renders an attribute description as a docstring", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: {
        symbol: makeSymbol({
          name: "Thing",
          kind: "class",
          docstring: "",
          attributes: [
            {
              name: "logger",
              description: "A :class:`Logger` for ``app``.\n\nSecond paragraph.",
              is_property: true,
              writable: false,
            },
          ],
        }),
        isActive: false,
      },
    });
    const doc = wrapper.find(".docstring-content");
    expect(doc.exists()).toBe(true);
    expect(doc.findAll("code").map((c) => c.text())).toEqual(["Logger", "app"]);
    expect(doc.findAll("p").length).toBe(2);
    expect(doc.text()).not.toContain(":class:");
  });

  it("renders no docstring block for an attribute without a description", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: {
        symbol: makeSymbol({
          name: "Thing",
          kind: "class",
          docstring: "",
          attributes: [{ name: "x", description: "", is_property: false, writable: false }],
        }),
        isActive: false,
      },
    });
    expect(wrapper.find(".docstring-content").exists()).toBe(false);
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
});
