import { describe, it, expect } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import PySignature from "../PySignature.vue";
import type { DocSymbol } from "~/types/api";

function makeSymbol(overrides: Partial<DocSymbol> = {}): DocSymbol {
  return {
    name: "hello",
    kind: "function",
    signature: "def hello(name: str) -> str",
    docstring: "",
    parameters: [{ name: "name", type: "str", description: "", kind: "positional_or_keyword" }],
    returns: { type: "str", description: "" },
    ...overrides,
  };
}

describe("PySignature", () => {
  it("renders function keyword", async () => {
    const wrapper = await mountSuspended(PySignature, { props: { symbol: makeSymbol() } });
    const keywords = wrapper.findAll(".py-keyword");
    expect(keywords.length).toBeGreaterThan(0);
    expect(keywords[0]!.text()).toBe("def");
  });

  it("renders function name", async () => {
    const wrapper = await mountSuspended(PySignature, { props: { symbol: makeSymbol() } });
    expect(wrapper.find(".py-name").text()).toBe("hello");
  });

  it("renders parameter name", async () => {
    const wrapper = await mountSuspended(PySignature, { props: { symbol: makeSymbol() } });
    expect(wrapper.find(".py-param").text()).toBe("name");
  });

  it("renders type annotation", async () => {
    const wrapper = await mountSuspended(PySignature, { props: { symbol: makeSymbol() } });
    const types = wrapper.findAll(".py-type");
    expect(types.some((t) => t.text() === "str")).toBe(true);
  });

  it("renders return arrow", async () => {
    const wrapper = await mountSuspended(PySignature, { props: { symbol: makeSymbol() } });
    expect(wrapper.text()).toContain("->");
  });

  it("renders class with bases", async () => {
    const wrapper = await mountSuspended(PySignature, {
      props: {
        symbol: {
          name: "MyError",
          kind: "class" as const,
          signature: "class MyError(ValueError)",
          docstring: "",
          parameters: [],
        },
      },
    });
    expect(wrapper.findAll(".py-keyword")[0]!.text()).toBe("class");
    expect(wrapper.find(".py-name").text()).toBe("MyError");
    expect(wrapper.find(".py-type").text()).toBe("ValueError");
  });

  it("renders default value", async () => {
    const wrapper = await mountSuspended(PySignature, {
      props: {
        symbol: makeSymbol({
          parameters: [
            {
              name: "timeout",
              type: "float",
              description: "",
              kind: "keyword_only",
              default: "30.0",
            },
          ],
        }),
      },
    });
    expect(wrapper.find(".py-default").text()).toBe("30.0");
  });

  it("renders var keyword params", async () => {
    const wrapper = await mountSuspended(PySignature, {
      props: {
        symbol: makeSymbol({
          parameters: [{ name: "kwargs", description: "", kind: "var_keyword" }],
        }),
      },
    });
    expect(wrapper.text()).toContain("**kwargs");
  });
});
