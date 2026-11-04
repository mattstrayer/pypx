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
    importableAs: [],
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

  it("shows the defining module only when the symbol has an origin", async () => {
    const plain = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol(), isActive: false },
    });
    expect(plain.find("[data-testid='symbol-origin']").exists()).toBe(false);

    const copied = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol({ origin: "sklearn._base" }), isActive: false },
    });
    const line = copied.find("[data-testid='symbol-origin']");
    expect(line.text()).toBe("Defined in sklearn._base");
    expect(line.find("a").exists()).toBe(false);
  });

  it("lists every alias when five or fewer exist", async () => {
    const plain = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol(), isActive: false },
    });
    expect(plain.find("[data-testid='symbol-aliases']").exists()).toBe(false);

    const paths = ["a.fit", "b.fit", "c.fit", "d.fit", "e.fit"];
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol({ importableAs: paths }), isActive: false },
    });
    const line = wrapper.find("[data-testid='symbol-aliases']");
    expect(line.text().replace(/\s+/g, " ")).toBe(
      "Also importable as a.fit, b.fit, c.fit, d.fit, e.fit",
    );
    expect(wrapper.find("[data-testid='symbol-aliases-toggle']").exists()).toBe(false);
  });

  it("folds aliases past the fifth behind a +N more button", async () => {
    const paths = ["a.fit", "b.fit", "c.fit", "d.fit", "e.fit", "f.fit", "g.fit"];
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol({ importableAs: paths }), isActive: false },
    });
    const line = wrapper.find("[data-testid='symbol-aliases']");
    const toggle = wrapper.find("[data-testid='symbol-aliases-toggle']");
    expect(toggle.text()).toBe("+2 more");
    expect(toggle.attributes("aria-expanded")).toBe("false");
    expect(line.text()).toContain("e.fit");
    expect(line.text()).not.toContain("f.fit");

    await toggle.trigger("click");
    expect(line.text()).toContain("g.fit");
    expect(toggle.attributes("aria-expanded")).toBe("true");

    await toggle.trigger("click");
    expect(line.text()).not.toContain("f.fit");
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

  it("shows one badge per fact beside the name", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: {
        symbol: makeSymbol({ decorators: ["abstractmethod", "final", "contextmanager"] }),
        isActive: false,
      },
    });
    expect(wrapper.findAll("[data-testid='symbol-badge']").map((b) => b.text())).toEqual([
      "abstract",
      "final",
      "contextmanager",
    ]);
  });

  it("does not repeat a classmethod label on a method row", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: {
        symbol: makeSymbol({
          name: "Pipeline",
          kind: "class",
          methods: [
            makeSymbol({
              name: "from_steps",
              method_kind: "classmethod",
              decorators: ["abstractmethod"],
            }),
          ],
        }),
        isActive: false,
      },
    });
    await wrapper.find("button").trigger("click");
    expect(wrapper.findAll("[data-testid='symbol-badge']").map((b) => b.text())).toEqual([
      "classmethod",
      "abstract",
    ]);
  });

  it("shows no badges on a symbol without decorators", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol(), isActive: false },
    });
    expect(wrapper.find("[data-testid='symbol-badge']").exists()).toBe(false);
  });

  it("folds overloads under the signature, closed by default", async () => {
    const overloads = ["def fit(X: int)", "def fit(X: str, y: int) -> str"];
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol({ overloads }), isActive: false },
    });
    const toggle = wrapper.find("[data-testid='symbol-overloads-toggle']");
    expect(toggle.text()).toBe("2 overloads");
    expect(toggle.attributes("aria-expanded")).toBe("false");
    expect(wrapper.find("[data-testid='symbol-overload']").exists()).toBe(false);

    await toggle.trigger("click");
    expect(wrapper.findAll("[data-testid='symbol-overload']").map((o) => o.text())).toEqual(
      overloads,
    );
    // The implementation stays the main line.
    expect(wrapper.text()).toContain("def fit");
  });

  it("says 1 overload, and shows no fold without overloads", async () => {
    const one = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol({ overloads: ["def fit(X: int)"] }), isActive: false },
    });
    expect(one.find("[data-testid='symbol-overloads-toggle']").text()).toBe("1 overload");

    const none = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol(), isActive: false },
    });
    expect(none.find("[data-testid='symbol-overloads']").exists()).toBe(false);
  });

  it("gives methods the same overload fold", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: {
        symbol: makeSymbol({
          name: "Pipeline",
          kind: "class",
          methods: [makeSymbol({ name: "run", overloads: ["def run(a: int)"] })],
        }),
        isActive: false,
      },
    });
    await wrapper.find("button").trigger("click");
    await wrapper.find("[data-testid='symbol-overloads-toggle']").trigger("click");
    expect(wrapper.find("[data-testid='symbol-overload']").text()).toBe("def run(a: int)");
  });

  it("folds nested classes after methods, closed by default", async () => {
    const inner = makeSymbol({
      name: "InvocationParams",
      kind: "class",
      signature: "class InvocationParams",
      decorators: ["dataclass", "final"],
      attributes: [{ name: "args", description: "", is_property: false, writable: false }],
      methods: [makeSymbol({ name: "explain" })],
      classes: [makeSymbol({ name: "Deep", kind: "class", signature: "class Deep" })],
    });
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: {
        symbol: makeSymbol({
          name: "Config",
          kind: "class",
          methods: [makeSymbol({ name: "getini" })],
          classes: [inner],
        }),
        isActive: false,
      },
    });
    const folds = () => wrapper.findAll("button").map((b) => b.text().replace(/\s+/g, " "));
    expect(folds()).toEqual(["Methods 1", "Nested classes 1"]);
    expect(wrapper.text()).not.toContain("InvocationParams");

    await wrapper.findAll("button")[1]!.trigger("click");
    const entry = wrapper.find("[data-testid='member-entry']");
    expect(entry.text()).toContain("InvocationParamsclassdataclassfinal");
    expect(entry.text()).toContain("class InvocationParams");
    expect(entry.text()).toContain("Attributes");
    // Its own members stay folded.
    expect(folds()).toEqual(["Methods 1", "Nested classes 1", "Methods 1", "Nested classes 1"]);
    expect(entry.text()).not.toContain("explain");
    expect(entry.text()).not.toContain("class Deep");

    await wrapper.findAll("button")[3]!.trigger("click");
    expect(entry.text()).toContain("class Deep");
  });

  it("shows no nested classes fold without nested classes", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol({ kind: "class", methods: [] }), isActive: false },
    });
    expect(wrapper.text()).not.toMatch(/nested classes/i);
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

  it("lets parameter, return and raise rows wrap at phone width", async () => {
    const fields = {
      parameters: [
        { name: "X", type: "array", description: "Data.", kind: "positional_or_keyword" },
      ],
      returns: { type: "self", description: "Done." },
      raises: [{ type: "ValueError", description: "Bad." }],
    };
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: {
        symbol: makeSymbol({
          kind: "class",
          ...fields,
          methods: [{ ...makeSymbol(), ...fields }],
        }),
        isActive: false,
      },
    });
    await wrapper.find("button[aria-expanded='false']").trigger("click");

    const rows = wrapper.findAll("[data-testid='field-row']");
    expect(rows).toHaveLength(6);
    for (const row of rows) expect(row.classes()).toContain("flex-wrap");
  });
});
