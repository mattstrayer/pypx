import { describe, it, expect, vi, beforeEach } from "vitest";
import { mountSuspended, mockNuxtImport } from "@nuxt/test-utils/runtime";
import ErrorPage from "../error.vue";

const { clearError } = vi.hoisted(() => ({ clearError: vi.fn() }));

mockNuxtImport("clearError", () => clearError);

const stubs = { NuxtLayout: { template: "<div><slot /></div>" }, CommandPalette: true };

beforeEach(() => {
  clearError.mockReset();
});

describe("error.vue", () => {
  it("shows a 404 message and clears the error on click", async () => {
    const wrapper = await mountSuspended(ErrorPage, {
      props: { error: { status: 404 } as never },
      global: { stubs },
    });

    expect(wrapper.text()).toContain("404");
    expect(wrapper.get("h1").text()).toBe("Page not found");

    await wrapper.get("button").trigger("click");
    expect(clearError).toHaveBeenCalledWith({ redirect: "/" });
  });

  it("does not show the error's message, which can hold internal detail", async () => {
    const wrapper = await mountSuspended(ErrorPage, {
      props: { error: { status: 500, statusText: "secret detail" } as never },
      global: { stubs },
    });

    expect(wrapper.text()).toContain("500");
    expect(wrapper.get("h1").text()).toBe("Something went wrong");
    expect(wrapper.text()).not.toContain("secret detail");
  });

  it("reads the older statusCode field", async () => {
    const wrapper = await mountSuspended(ErrorPage, {
      props: { error: { statusCode: 404 } as never },
      global: { stubs },
    });

    expect(wrapper.get("h1").text()).toBe("Page not found");
  });
});
