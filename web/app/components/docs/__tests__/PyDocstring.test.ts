import { describe, it, expect } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import PyDocstring from "../PyDocstring.vue";

describe("PyDocstring", () => {
  it("renders nothing for empty text", async () => {
    const wrapper = await mountSuspended(PyDocstring, { props: { text: "" } });
    expect(wrapper.find(".docstring-content").exists()).toBe(false);
  });

  it("renders paragraphs", async () => {
    const wrapper = await mountSuspended(PyDocstring, {
      props: { text: "Hello world.\n\nSecond paragraph." },
    });
    const ps = wrapper.findAll("p");
    expect(ps.length).toBeGreaterThanOrEqual(2);
  });

  it("renders code spans", async () => {
    const wrapper = await mountSuspended(PyDocstring, {
      props: { text: "Use ``foo`` for this." },
    });
    expect(wrapper.find("code").text()).toBe("foo");
  });

  it("renders code blocks", async () => {
    const wrapper = await mountSuspended(PyDocstring, {
      props: { text: "Example::\n\n    foo()\n    bar()" },
    });
    // In test environment (not SSR), code blocks won't have Shiki highlighting
    // but should still be rendered as pre/code elements
    expect(wrapper.find("pre").exists()).toBe(true);
  });
});
