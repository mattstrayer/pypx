export default defineEventHandler((event) => {
  const config = useRuntimeConfig();
  // event.path includes the /api prefix (e.g. /api/popular), but config.apiBase
  // already ends with /api (e.g. http://api:8080/api), so strip the leading /api.
  const path = event.path.replace(/^\/api/, '');
  return proxyRequest(event, `${config.apiBase}${path}`);
});
