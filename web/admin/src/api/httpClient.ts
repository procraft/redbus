import axios, { isAxiosError } from 'axios';

import config from '@/config';

const httpClient = axios.create({
  baseURL: config.apiBaseUrl,
  headers: {
    'Content-Type': 'application/json',
    ...(config.apiToken ? { Authorization: `Token ${config.apiToken}` } : {}),
  },
});

export function getRequestErrorMessage(error: unknown): string {
  if (isAxiosError<{ error?: string }>(error)) {
    return error.response?.data?.error ?? error.message;
  }

  return error instanceof Error ? error.message : 'Unknown request error';
}

export default httpClient;
