declare const __REDBUS_API_HOST__: string;
declare const __REDBUS_API_TOKEN__: string;

interface Window {
  __REDBUS_RUNTIME_CONFIG__?: {
    apiHost?: string;
  };
}

declare module '*.css';
