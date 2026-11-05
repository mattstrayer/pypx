import { describe, it, expect } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import SearchDropdown from "../SearchDropdown.vue";
import type { SearchResult } from "~/types/api";

function makeResult(name: string): SearchResult {
  return { name, summary: `${name} summary`, downloads: 1000 };
}

describe("SearchDropdown", () => {
  it("wires listbox and option ids and aria-selected from listboxId", async () => {
    const results = [makeResult("requests"), makeResult("httpx")];
    const wrapper = await mountSuspended(SearchDropdown, {
      props: {
        results,
        selectedIndex: 0,
        loading: false,
        hasQuery: true,
        listboxId: "test-lb",
      },
    });

    const listbox = wrapper.find('[role="listbox"]');
    expect(listbox.exists()).toBe(true);
    expect(listbox.attributes("id")).toBe("test-lb");

    const options = wrapper.findAll('[role="option"]');
    expect(options).toHaveLength(2);
    expect(options[0]!.attributes("id")).toBe("test-lb-opt-0");
    expect(options[1]!.attributes("id")).toBe("test-lb-opt-1");
    expect(options[0]!.attributes("aria-selected")).toBe("true");
    expect(options[1]!.attributes("aria-selected")).toBe("false");
  });

  it("omits ids when no listboxId is provided", async () => {
    const wrapper = await mountSuspended(SearchDropdown, {
      props: {
        results: [makeResult("requests")],
        selectedIndex: -1,
        loading: false,
        hasQuery: true,
      },
    });

    const listbox = wrapper.find('[role="listbox"]');
    expect(listbox.attributes("id")).toBeUndefined();
    const option = wrapper.find('[role="option"]');
    expect(option.attributes("id")).toBeUndefined();
  });
});
