export const ETHICALADS_CLIENT_SRC = "https://media.ethicalads.io/media/client/ethicalads.min.js";

export type AdType = "image" | "text";

interface EthicalAdsClient {
  reload: () => void;
}

function getClient(): EthicalAdsClient | undefined {
  return (globalThis as { ethicalads?: EthicalAdsClient }).ethicalads;
}

export function useEthicalAds() {
  const config = useRuntimeConfig();
  const ads = config.public.ads as { publisher?: string; type?: string } | undefined;

  const publisher = ads?.publisher ?? "";
  const type: AdType = ads?.type === "text" ? "text" : "image";
  const enabled = Boolean(publisher);

  // Script injection lives in `plugins/ethicalads.client.ts`, not here: a
  // component-scoped head entry is disposed on unmount and would double-load
  // the vendor client every time an AdSlot remounts.

  // No-op until the vendor client has executed. That is the correct contract for
  // a placement mounted before the script loads: the script's own initial scan
  // will find it. A placement mounted afterwards is only discovered by this call.
  function reload(): void {
    getClient()?.reload();
  }

  return { enabled, publisher, type, reload };
}
