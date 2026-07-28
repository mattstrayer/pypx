import { describe, it, expect } from "vitest";
import { mountSuspended, mockNuxtImport } from "@nuxt/test-utils/runtime";
import { reactive } from "vue";
import DefaultLayout from "../../layouts/default.vue";

const routeMock = reactive({
  path: "/packages/requests",
  fullPath: "/packages/requests",
  query: {},
});

mockNuxtImport("useRoute", () => () => routeMock);

describe("default layout agent discovery footer link", () => {
  it("renders an API for agents link pointing at /llms.txt", async () => {
    const wrapper = await mountSuspended(DefaultLayout);

    const link = wrapper.find('a[href="/llms.txt"]');
    expect(link.exists()).toBe(true);
    expect(link.text()).toBe("API for agents");
  });
});
