import { describe, it, expect, vi, beforeEach } from "vitest";
import { mountSuspended, mockNuxtImport } from "@nuxt/test-utils/runtime";
import { reactive } from "vue";
import PackagePage from "../packages/[name].vue";

const routeMock = reactive({
  params: { name: "nope" },
  path: "/packages/nope",
  fullPath: "/packages/nope",
  query: {},
});

mockNuxtImport("useRoute", () => () => routeMock);

const { createErrorSpy } = vi.hoisted(() => ({ createErrorSpy: vi.fn() }));

// Nuxt's own useAsyncData internals also call createError() (to normalize
// error.value) — it must construct/return an error, not throw. The page code
// is responsible for `throw createError(...)`.
mockNuxtImport(
  "createError",
  () => (input: { statusCode?: number; statusMessage?: string } | Error) => {
    createErrorSpy(input);
    if (input instanceof Error) return input;
    return Object.assign(new Error(input.statusMessage ?? "error"), input);
  },
);

function makePackage(name: string) {
  return {
    name,
    version: "1.0.0",
    summary: "",
    description: "",
    description_content_type: "",
    description_html: "",
    license: "MIT",
    author: "",
    author_email: "",
    home_page: "",
    requires_python: "",
    requires_dist: [],
    project_urls: {},
    classifiers: [],
    latest_files: [],
    install_size: 0,
    module_format: "",
    python_versions: { constraint: "", min_version: "" },
    dependencies: { required: [], extras: {} },
    platform_coverage: {
      pure_python: true,
      linux_x86_64: false,
      linux_arm64: false,
      macos_x86_64: false,
      macos_arm64: false,
      windows_x86_64: false,
      musl: false,
    },
    release_cadence: {
      releases_last_12mo: 0,
      avg_days_between_releases: 0,
      last_released_at: "",
      quarterly_counts: [],
    },
    maintainers: [],
    doc_url: "",
  };
}

const fetchPackage = vi.fn();
const fetchSecurity = vi.fn().mockResolvedValue(null);
const fetchExtras = vi.fn().mockResolvedValue(null);
const fetchChangelog = vi.fn().mockResolvedValue(null);
const fetchDocs = vi.fn().mockResolvedValue(null);

mockNuxtImport("useApi", () => () => ({
  fetchPackage,
  fetchSecurity,
  fetchExtras,
  fetchChangelog,
  fetchDocs,
}));

beforeEach(() => {
  fetchPackage.mockReset();
  fetchSecurity.mockReset().mockResolvedValue(null);
  fetchExtras.mockReset().mockResolvedValue(null);
  fetchChangelog.mockReset().mockResolvedValue(null);
  fetchDocs.mockReset().mockResolvedValue(null);
  createErrorSpy.mockReset();
  vi.stubGlobal("defineOgImage", () => undefined);
  vi.stubGlobal("useSchemaOrg", () => undefined);
});

describe("packages/[name].vue real 404 handling", () => {
  it("throws a 404 error when the package does not exist", async () => {
    fetchPackage.mockRejectedValue(Object.assign(new Error("not found"), { statusCode: 404 }));

    await expect(mountSuspended(PackagePage)).rejects.toThrow();
    expect(createErrorSpy).toHaveBeenCalledWith(expect.objectContaining({ statusCode: 404 }));
  });

  it("throws a 502 error when the upstream fetch fails for another reason", async () => {
    fetchPackage.mockRejectedValue(Object.assign(new Error("upstream down"), { statusCode: 502 }));

    await expect(mountSuspended(PackagePage)).rejects.toThrow();
    expect(createErrorSpy).toHaveBeenCalledWith(expect.objectContaining({ statusCode: 502 }));
  });

  it("does not 404 when only client-only fetches fail on an existing package", async () => {
    fetchPackage.mockResolvedValue(makePackage("nope"));
    fetchSecurity.mockRejectedValue(new Error("security unavailable"));
    fetchExtras.mockRejectedValue(new Error("extras unavailable"));

    const wrapper = await mountSuspended(PackagePage);

    await vi.waitFor(() => {
      expect(wrapper.text()).toContain("nope");
    });
  });
});
