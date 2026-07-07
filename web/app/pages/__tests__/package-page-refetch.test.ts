import { describe, it, expect, vi } from "vitest";
import { mountSuspended, mockNuxtImport } from "@nuxt/test-utils/runtime";
import { reactive } from "vue";
import PackagePage from "../packages/[name].vue";

const routeMock = reactive({
  params: { name: "left" },
  path: "/packages/left",
  fullPath: "/packages/left",
  query: {},
});

mockNuxtImport("useRoute", () => () => routeMock);

const fetchPackage = vi.fn(async (name: string) => ({
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
}));
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

describe("packages/[name].vue refetch on client-side navigation", () => {
  it("refetches package data when the route name param changes", async () => {
    vi.stubGlobal("defineOgImage", () => undefined);
    vi.stubGlobal("useSchemaOrg", () => undefined);

    const wrapper = await mountSuspended(PackagePage);

    await vi.waitFor(() => {
      expect(wrapper.text()).toContain("left");
    });

    routeMock.params.name = "right";
    routeMock.path = "/packages/right";
    routeMock.fullPath = "/packages/right";

    await vi.waitFor(() => {
      expect(fetchPackage).toHaveBeenCalledWith("right");
    });

    await vi.waitFor(() => {
      expect(wrapper.text()).toContain("right");
    });
  });
});
