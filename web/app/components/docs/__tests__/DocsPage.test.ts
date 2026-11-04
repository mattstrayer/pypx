import { describe, it, expect, beforeEach, vi } from "vitest";
import { flushPromises } from "@vue/test-utils";
import { mockNuxtImport, mountSuspended } from "@nuxt/test-utils/runtime";
import DocsPage from "~/pages/packages/[name]/docs.vue";
import type { DocsResponse } from "~/types/api";

const fetchDocs = vi.hoisted(() => vi.fn());

mockNuxtImport("useApi", () => () => ({
  fetchPackage: async () => ({ name: "pkg", version: "1.0", summary: "", doc_url: "" }),
  fetchDocs,
}));

mockNuxtImport("useRoute", () => () => ({
  params: { name: "pkg" },
  query: {},
  path: "/packages/pkg/docs",
}));

// The test build leaves out the OG image module, so its auto-import does not exist.
vi.stubGlobal("defineOgImage", () => {});

describe("docs page", () => {
  beforeEach(() => {
    clearNuxtData();
    fetchDocs.mockReset();
  });

  it("offers no symbol jump when the release has no docs", async () => {
    const unavailable: DocsResponse = {
      package: "pkg",
      version: "1.0",
      available: false,
      reason: "no_artifact",
      modules: [],
    };
    fetchDocs.mockResolvedValue(unavailable);
    const wrapper = await mountSuspended(DocsPage);
    await flushPromises();
    expect(wrapper.text()).toContain("API documentation is not available");
    expect(wrapper.text()).not.toContain("Jump to symbol");
  });

  it("offers the symbol jump when docs exist", async () => {
    const available: DocsResponse = {
      package: "pkg",
      version: "1.0",
      available: true,
      modules: [
        {
          name: "pkg",
          functions: [
            { name: "f", kind: "function", signature: "def f()", docstring: "", parameters: [] },
          ],
          classes: [],
          exceptions: [],
        },
      ],
    };
    fetchDocs.mockResolvedValue(available);
    const wrapper = await mountSuspended(DocsPage);
    await flushPromises();
    expect(wrapper.text()).toContain("Jump to symbol");
  });
});
