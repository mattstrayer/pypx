import { describe, it, expect, vi, beforeEach } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import PyDocstring from "../PyDocstring.vue";

const highlightPython = vi.hoisted(() => vi.fn());
vi.mock("~/utils/shikiHighlight", () => ({ highlightPython }));

const WITH_CODE = "Example::\n\n    foo()";

describe("PyDocstring highlighting", () => {
  beforeEach(() => {
    highlightPython.mockReset();
    highlightPython.mockImplementation(
      async (code: string) => `<pre class="shiki"><code>${code}</code></pre>`,
    );
  });

  it("replaces plain code with highlighted code", async () => {
    const wrapper = await mountSuspended(PyDocstring, { props: { text: WITH_CODE } });
    await vi.waitFor(() => expect(wrapper.find("pre.shiki").exists()).toBe(true));
    expect(highlightPython).toHaveBeenCalledWith("foo()");
    expect(wrapper.find("code.language-python").exists()).toBe(false);
  });

  it("shows plain code until the highlighter finishes", async () => {
    highlightPython.mockImplementation(() => new Promise(() => {}));
    const wrapper = await mountSuspended(PyDocstring, { props: { text: WITH_CODE } });
    expect(wrapper.find("code.language-python").text()).toBe("foo()");
  });

  it("keeps plain code when the highlighter fails", async () => {
    highlightPython.mockRejectedValue(new Error("chunk failed"));
    const wrapper = await mountSuspended(PyDocstring, { props: { text: WITH_CODE } });
    await vi.waitFor(() => expect(highlightPython).toHaveBeenCalled());
    await new Promise((resolve) => setTimeout(resolve));
    expect(wrapper.find("code.language-python").text()).toBe("foo()");
  });

  it("does not call the highlighter for a docstring with no code", async () => {
    const wrapper = await mountSuspended(PyDocstring, { props: { text: "Only prose." } });
    await new Promise((resolve) => setTimeout(resolve));
    expect(wrapper.find("p").text()).toBe("Only prose.");
    expect(highlightPython).not.toHaveBeenCalled();
  });

  it("never shows the old result after the text changes", async () => {
    const wrapper = await mountSuspended(PyDocstring, { props: { text: WITH_CODE } });
    await vi.waitFor(() => expect(wrapper.find("pre.shiki").exists()).toBe(true));
    highlightPython.mockImplementation(() => new Promise(() => {}));
    await wrapper.setProps({ text: "Example::\n\n    bar()" });
    expect(wrapper.text()).toContain("bar()");
    expect(wrapper.text()).not.toContain("foo()");
  });
});
