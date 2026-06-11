import { describe, it, expect } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import DiffSection from "../DiffSection.vue";

describe("DiffSection", () => {
  it("renders the section title", async () => {
    const wrapper = await mountSuspended(DiffSection, {
      props: { title: "API Changes" },
    });
    expect(wrapper.text()).toContain("API Changes");
  });

  it("shows unavailable message when set", async () => {
    const wrapper = await mountSuspended(DiffSection, {
      props: { title: "Changelog", unavailable: "no github token" },
    });
    expect(wrapper.text()).toContain("no github token");
  });

  it("shows empty message when empty is true", async () => {
    const wrapper = await mountSuspended(DiffSection, {
      props: { title: "Deps", empty: true, emptyMessage: "No dependency changes." },
    });
    expect(wrapper.text()).toContain("No dependency changes.");
  });

  it("renders slot content when data is present", async () => {
    const wrapper = await mountSuspended(DiffSection, {
      props: { title: "API Changes", empty: false },
      slots: { default: "<div>+ added: httpx.Client</div>" },
    });
    expect(wrapper.text()).toContain("httpx.Client");
  });
});
