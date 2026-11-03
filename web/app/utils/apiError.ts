/**
 * Server-side API calls use a private URL. A raw FetchError holds that URL in its
 * message and fields, and Nuxt serializes useAsyncData errors into the page payload.
 */
export function toPublicApiError(err: unknown) {
  const statusCode = (err as { statusCode?: number } | null)?.statusCode ?? 503;
  return createError({ statusCode, statusMessage: "API request failed" });
}

/** The HTTP status for a page whose API call failed, or null if it did not fail. */
export function pageStatusForApiError(
  err: { statusCode?: number } | null | undefined,
): 404 | 503 | null {
  if (!err) return null;
  return err.statusCode === 404 || err.statusCode === 400 ? 404 : 503;
}
