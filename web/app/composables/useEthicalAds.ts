const CLIENT_SRC = "https://media.ethicalads.io/media/client/ethicalads.min.js";
const CLIENT_TIMEOUT_MS = 5000;
const POLL_INTERVAL_MS = 100;

export type AdType = "image" | "text";

interface EthicalAdsPlacement {
  response?: { campaign_type?: string };
}

interface EthicalAdsClient {
  wait: Promise<EthicalAdsPlacement[]>;
  reload: () => void;
}

function getClient(): EthicalAdsClient | undefined {
  return (globalThis as { ethicalads?: EthicalAdsClient }).ethicalads;
}

/**
 * Waits for the async-loaded vendor global to appear. Resolves undefined on
 * timeout, which callers treat identically to a no-fill — so a blocked or
 * failed script degrades exactly like empty inventory.
 */
function waitForClient(): Promise<EthicalAdsClient | undefined> {
  const existing = getClient();
  if (existing) return Promise.resolve(existing);

  return new Promise((resolve) => {
    const deadline = Date.now() + CLIENT_TIMEOUT_MS;
    const timer = setInterval(() => {
      const client = getClient();
      if (client) {
        clearInterval(timer);
        resolve(client);
      } else if (Date.now() >= deadline) {
        clearInterval(timer);
        resolve(undefined);
      }
    }, POLL_INTERVAL_MS);
  });
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

  async function waitForFill(): Promise<boolean> {
    if (!enabled || !import.meta.client) return false;
    const client = await waitForClient();
    if (!client) return false;
    try {
      const placements = await client.wait;
      return Array.isArray(placements) && placements.length > 0;
    } catch {
      return false;
    }
  }

  function reload(): void {
    getClient()?.reload();
  }

  return { enabled, publisher, type, waitForFill, reload };
}
