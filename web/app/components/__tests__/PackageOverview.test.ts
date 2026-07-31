import { describe, it, expect, beforeEach, vi } from "vitest";
import { ref, createSSRApp, h } from "vue";
import { renderToString } from "vue/server-renderer";
import { mountSuspended, mockNuxtImport } from "@nuxt/test-utils/runtime";
import type { PackageData } from "~/types/api";
import PackageOverview from "../PackageOverview.vue";

const isDesktop = ref(true);
vi.mock("@vueuse/core", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@vueuse/core")>();
  return { ...actual, useMediaQuery: () => isDesktop };
});

const mockAds = {
  enabled: true,
  publisher: "pypx",
  type: "image" as const,
  reload: vi.fn(),
};
mockNuxtImport("useEthicalAds", () => () => mockAds);

const pkg: PackageData = {
  name: "requests",
  version: "2.32.3",
  summary: "HTTP for Humans",
  description: "readme body",
  description_content_type: "text/markdown",
  description_html: "<p>readme body</p>",
  license: "",
  author: "",
  author_email: "",
  home_page: "",
  requires_python: "",
  requires_dist: [],
  classifiers: ["Topic :: Internet :: WWW/HTTP", "License :: OSI Approved :: MIT License"],
  project_urls: {},
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

describe("PackageOverview ad slot", () => {
  beforeEach(() => {
    isDesktop.value = true;
  });

  it("renders exactly one placement on desktop", async () => {
    const wrapper = await mountSuspended(PackageOverview, { props: { pkg } });
    expect(wrapper.findAll("[data-ea-publisher]")).toHaveLength(1);
  });

  it("renders exactly one placement on mobile", async () => {
    isDesktop.value = false;
    const wrapper = await mountSuspended(PackageOverview, { props: { pkg } });
    expect(wrapper.findAll("[data-ea-publisher]")).toHaveLength(1);
  });

  it("passes keywords derived from classifiers", async () => {
    const wrapper = await mountSuspended(PackageOverview, { props: { pkg } });
    expect(wrapper.find("[data-ea-publisher]").attributes("data-ea-keywords")).toBe("www/http");
  });

  it("renders no placement in server-rendered HTML (avoids a hydration mismatch)", async () => {
    // useMediaQuery resolves to false during real SSR (no window), so the
    // pre-mount server output must contain neither ad slot regardless of
    // which viewport branch isDesktop would eventually pick on the client.
    const app = createSSRApp({ render: () => h(PackageOverview, { pkg }) });
    const html = await renderToString(app);
    expect(html).not.toContain("data-ea-publisher");
  });
});
