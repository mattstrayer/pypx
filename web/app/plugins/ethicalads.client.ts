import { ETHICALADS_CLIENT_SRC } from "../composables/useEthicalAds";

// The vendor client must be injected exactly once for the lifetime of the app.
// It is deliberately NOT injected from a component: a component-scoped `useHead`
// entry is disposed on unmount and re-injected on the next mount, which
// double-loads the client (the vendor explicitly warns against this and logs a
// warning). Owning the tag here means the script survives every AdSlot
// unmount/remount — breakpoint swaps, and leaving and returning to a package
// page — and placements mounted after it executed are discovered via reload().
export default defineNuxtPlugin(() => {
  const config = useRuntimeConfig();
  const ads = config.public.ads as { publisher?: string } | undefined;

  if (!ads?.publisher) return;

  useHead({
    script: [{ src: ETHICALADS_CLIENT_SRC, async: true, key: "ethicalads" }],
  });
});
