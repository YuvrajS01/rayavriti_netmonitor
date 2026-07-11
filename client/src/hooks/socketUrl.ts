const DEFAULT_WS_PATH = '/api/v1/ws';

const getWebSocketProtocol = () => (window.location.protocol === 'https:' ? 'wss:' : 'ws:');

export function resolveWebSocketUrl(configuredUrl = import.meta.env.VITE_WS_URL): string {
  const protocol = getWebSocketProtocol();

  if (!configuredUrl) {
    return new URL(DEFAULT_WS_PATH, `${protocol}//${window.location.host}`).toString();
  }

  if (/^(wss?|https?):\/\//i.test(configuredUrl)) {
    const url = new URL(configuredUrl);
    if (url.protocol === 'https:') url.protocol = 'wss:';
    if (url.protocol === 'http:') url.protocol = 'ws:';
    return url.toString();
  }

  if (configuredUrl.startsWith('/')) {
    return new URL(configuredUrl, `${protocol}//${window.location.host}`).toString();
  }

  if (configuredUrl.includes('/')) {
    return new URL(`${protocol}//${configuredUrl}`).toString();
  }

  return new URL(DEFAULT_WS_PATH, `${protocol}//${configuredUrl}`).toString();
}
