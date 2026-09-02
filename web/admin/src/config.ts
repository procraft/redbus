const runtimeApiHost = window.__REDBUS_RUNTIME_CONFIG__?.apiHost?.trim();
const apiHost = (runtimeApiHost || __REDBUS_API_HOST__).replace(/\/$/, '');

const config = {
  apiHost,
  apiToken: __REDBUS_API_TOKEN__,
  apiBaseUrl: `${apiHost}/api`,
} as const;

export default config;
