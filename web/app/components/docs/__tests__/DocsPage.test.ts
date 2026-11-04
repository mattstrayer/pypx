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

  it("has one h1 naming the package", async () => {
    fetchDocs.mockResolvedValue({ package: "pkg", version: "1.0", available: true, modules: [] });
    const page = await mountSuspended(DocsPage);
    await flushPromises();
    const headings = page.findAll("h1");
    expect(headings).toHaveLength(1);
    expect(headings[0]?.text()).toBe("pkg API documentation");
  });

  it("shows every symbol with a repeated name, each with its own card", async () => {
    const redis = (name: string) => ({
      name: "Redis",
      kind: "class" as const,
      signature: `class Redis  # ${name}`,
      docstring: "",
      parameters: [],
    });
    fetchDocs.mockResolvedValue({
      package: "pkg",
      version: "1.0",
      available: true,
      modules: [
        { name: "redis.asyncio.client", functions: [], classes: [redis("a")], exceptions: [] },
        { name: "redis.client", functions: [], classes: [redis("b")], exceptions: [] },
      ],
    });
    const page = await mountSuspended(DocsPage);
    await flushPromises();
    expect(page.find("[id='sym-redis.asyncio.client.Redis']").exists()).toBe(true);
    expect(page.find("[id='sym-redis.client.Redis']").exists()).toBe(true);
  });

  describe("re-exports", () => {
    const fn = (name: string) => ({
      name,
      kind: "function" as const,
      signature: `def ${name}()`,
      docstring: "",
      parameters: [],
    });

    it("lists every alias by module, as plain text, after the cards", async () => {
      fetchDocs.mockResolvedValue({
        package: "numpy",
        version: "1.0",
        available: true,
        modules: [
          { name: "numpy.char", functions: [fn("capitalize")], classes: [], exceptions: [] },
          {
            name: "numpy.strings",
            functions: [],
            classes: [],
            exceptions: [],
            aliases: [
              { name: "capitalize", target: "numpy.char.capitalize" },
              { name: "isalnum", target: "numpy.char.isalnum" },
            ],
          },
        ],
      });
      const page = await mountSuspended(DocsPage);
      await flushPromises();
      const section = page.find("[data-testid='re-exports']");
      expect(section.find("h2").text()).toBe("Re-exports");
      expect(section.find("h3").text()).toBe("numpy.strings");
      expect(section.findAll("li").map((li) => li.text().replace(/\s+/g, " "))).toEqual([
        "capitalize -> numpy.char.capitalize",
        "isalnum -> numpy.char.isalnum",
      ]);
      expect(section.find("a").exists()).toBe(false);

      // An alias with a card also has the card line. One with no card has no card.
      expect(page.find("[data-testid='symbol-aliases']").text()).toContain(
        "numpy.strings.capitalize",
      );
      expect(page.text()).not.toContain("numpy.strings.isalnum");
      const cards = page.find("[id='sym-numpy.char.capitalize']").element;
      expect(cards.compareDocumentPosition(section.element)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    });

    it("has no section when the package has no aliases", async () => {
      fetchDocs.mockResolvedValue({
        package: "pkg",
        version: "1.0",
        available: true,
        modules: [{ name: "pkg", functions: [fn("f")], classes: [], exceptions: [] }],
      });
      const page = await mountSuspended(DocsPage);
      await flushPromises();
      expect(page.find("[data-testid='re-exports']").exists()).toBe(false);
    });

    it("shows the section for a package with aliases and no cards", async () => {
      fetchDocs.mockResolvedValue({
        package: "pkg",
        version: "1.0",
        available: true,
        modules: [
          {
            name: "pkg",
            functions: [],
            classes: [],
            exceptions: [],
            aliases: [{ name: "add", target: "pkg.core.add" }],
          },
        ],
      });
      const page = await mountSuspended(DocsPage);
      await flushPromises();
      expect(page.find("[data-testid='re-exports']").text()).toContain("add -> pkg.core.add");
    });

    it("gives no two page elements one id", async () => {
      fetchDocs.mockResolvedValue({
        package: "pkg",
        version: "1.0",
        available: true,
        modules: [
          // One path twice, as a function and a class.
          {
            name: "pkg.a",
            functions: [fn("x")],
            classes: [{ ...fn("x"), kind: "class" }],
            exceptions: [],
          },
          { name: "pkg.b", functions: [fn("x")], classes: [], exceptions: [] },
          // Aliases that share a name with each other and with a card.
          {
            name: "pkg",
            functions: [],
            classes: [],
            exceptions: [],
            aliases: [{ name: "x", target: "pkg.a.x" }],
          },
          {
            name: "pkg.c",
            functions: [],
            classes: [],
            exceptions: [],
            aliases: [
              { name: "x", target: "pkg.a.x" },
              { name: "y", target: "pkg.attrs.y" },
            ],
          },
        ],
      });
      const page = await mountSuspended(DocsPage);
      await flushPromises();
      const ids = page.findAll("[id]").map((el) => el.attributes("id"));
      expect(ids.filter((id) => id?.startsWith("sym-"))).toHaveLength(3);
      expect(ids).toContain("re-exports");
      expect(new Set(ids).size).toBe(ids.length);
    });
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
