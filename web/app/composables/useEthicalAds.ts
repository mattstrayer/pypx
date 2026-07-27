const CLIENT_SRC = "https://media.ethicalads.io/media/client/ethicalads.min.js";

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

  if (enabled) {
    useHead({
      script: [{ src: CLIENT_SRC, async: true, key: "ethicalads" }],
    });
  }

  function reload(): void {
    getClient()?.reload();
  }

  return { enabled, publisher, type, reload };
}
