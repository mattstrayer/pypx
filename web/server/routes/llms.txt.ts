export default defineEventHandler((event) => {
  const config = useRuntimeConfig();
  // config.apiBase points at the API's /api prefix (e.g. http://api:8080/api).
  // In production Caddy routes /llms.txt straight to the API, but `make
  // dev-web` only proxies /api/*, so mirror that route here for local dev.
  const origin = config.apiBase.replace(/\/api\/?$/, '');
  return proxyRequest(event, `${origin}/llms.txt`);
});
