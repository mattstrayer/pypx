import { describe, it, expect, vi, beforeEach } from "vitest";
import { mountSuspended, mockNuxtImport } from "@nuxt/test-utils/runtime";
import ErrorPage from "../error.vue";

const { clearError } = vi.hoisted(() => ({ clearError: vi.fn() }));

mockNuxtImport("clearError", () => clearError);

beforeEach(() => {
  clearError.mockReset();
});

describe("error.vue", () => {
  it("renders a 404 message and clears the error on click", async () => {
    const wrapper = await mountSuspended(ErrorPage, {
      props: {
        error: { statusCode: 404, statusMessage: "Package not found" } as never,
      },
      global: {
        stubs: { NuxtLayout: { template: "<div><slot /></div>" } },
      },
    });

    expect(wrapper.text()).toContain("404");
    expect(wrapper.text()).toContain("Package not found");

    await wrapper.find("button").trigger("click");
    expect(clearError).toHaveBeenCalledWith({ redirect: "/" });
  });

  it("renders a generic message for non-404 errors", async () => {
    const wrapper = await mountSuspended(ErrorPage, {
      props: {
        error: { statusCode: 502, statusMessage: "Failed to load package" } as never,
      },
      global: {
        stubs: { NuxtLayout: { template: "<div><slot /></div>" } },
      },
    });

    expect(wrapper.text()).toContain("502");
    expect(wrapper.text()).toContain("Something went wrong");
  });
});
