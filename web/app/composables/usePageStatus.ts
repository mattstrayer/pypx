/**
 * Sends the page's failure status in the server response and keeps the page out of
 * search results. An outage must not stay in a shared cache, so 503 also sets no-store.
 */
export function usePageStatus(status: Ref<number | null>) {
  if (!import.meta.server || !status.value) return;
  const event = useRequestEvent();
  if (event) setResponseStatus(event, status.value);
  if (status.value >= 500) useResponseHeader("Cache-Control").value = "no-store";
  useRobotsRule(false);
}
