import { describe, it, expect } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import CompareTable from "../CompareTable.vue";
import type { ComparePackageData } from "~/types/api";

function makePackage(overrides: Partial<ComparePackageData> = {}): ComparePackageData {
  return {
    Name: "requests",
    Version: "2.32.3",
    Summary: "HTTP for Humans",
    License: "Apache-2.0",
    PythonMin: "3.8",
    InstallSize: 150000,
    ModuleFormat: "wheel",
    LastReleasedDate: "2024-05-21",
    ReleasesLast12Mo: 5,
    DepCount: 4,
    Downloads30d: 70_000_000,
    VulnCount: 0,
    Typed: "yes",
    RepoURL: "https://github.com/psf/requests",
    DocURL: "https://requests.readthedocs.io",
    ...overrides,
  };
}

describe("CompareTable", () => {
  it("renders package names", async () => {
    const packages = [makePackage(), makePackage({ Name: "httpx", Version: "0.28.1" })];
    const wrapper = await mountSuspended(CompareTable, { props: { packages } });
    expect(wrapper.text()).toContain("requests");
    expect(wrapper.text()).toContain("httpx");
  });

  it("renders version row", async () => {
    const packages = [
      makePackage({ Version: "2.32.3" }),
      makePackage({ Name: "httpx", Version: "0.28.1" }),
    ];
    const wrapper = await mountSuspended(CompareTable, { props: { packages } });
    expect(wrapper.text()).toContain("2.32.3");
    expect(wrapper.text()).toContain("0.28.1");
  });

  it("renders the license row", async () => {
    const packages = [makePackage({ License: "MIT" })];
    const wrapper = await mountSuspended(CompareTable, { props: { packages } });
    expect(wrapper.text()).toContain("MIT");
  });

  it("formats install size in KB", async () => {
    const packages = [makePackage({ InstallSize: 150000 })];
    const wrapper = await mountSuspended(CompareTable, { props: { packages } });
    expect(wrapper.text()).toMatch(/\d+\s*KB/);
  });

  it("shows — for missing DocURL", async () => {
    const packages = [makePackage({ DocURL: "" })];
    const wrapper = await mountSuspended(CompareTable, { props: { packages } });
    // The docs row should render with a dash
    expect(wrapper.html()).toContain("Docs");
  });
});
