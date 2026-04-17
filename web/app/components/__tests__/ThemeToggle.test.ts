import { describe, it, expect, beforeEach } from "vitest";
import { mountSuspended, mockNuxtImport } from "@nuxt/test-utils/runtime";
import ThemeToggle from "../ThemeToggle.vue";

const mockColorMode = { preference: "system", value: "dark" };

mockNuxtImport("useColorMode", () => () => mockColorMode);

describe("ThemeToggle", () => {
  beforeEach(() => {
    mockColorMode.preference = "system";
    mockColorMode.value = "dark";
  });

  it("cycles system → light on first click", async () => {
    const wrapper = await mountSuspended(ThemeToggle);
    await wrapper.find("button").trigger("click");
    expect(mockColorMode.preference).toBe("light");
  });

  it("cycles light → dark", async () => {
    mockColorMode.preference = "light";
    const wrapper = await mountSuspended(ThemeToggle);
    await wrapper.find("button").trigger("click");
    expect(mockColorMode.preference).toBe("dark");
  });

  it("cycles dark → system", async () => {
    mockColorMode.preference = "dark";
    const wrapper = await mountSuspended(ThemeToggle);
    await wrapper.find("button").trigger("click");
    expect(mockColorMode.preference).toBe("system");
  });
});
