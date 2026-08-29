import { notifications } from '@mantine/notifications';
import { useCallback, useState } from 'react';

import { getRequestErrorMessage } from '@/api/httpClient';

export function useRequest() {
  const [isLoading, setIsLoading] = useState(false);

  const execute = useCallback(
    async <Response,>(
      request: () => Promise<Response>,
      onSuccess?: (response: Response) => void,
      successMessage?: string,
    ): Promise<boolean> => {
      setIsLoading(true);

      try {
        const response = await request();
        onSuccess?.(response);

        if (successMessage) {
          notifications.show({ color: 'teal', message: successMessage });
        }

        return true;
      } catch (error) {
        console.error(error);
        notifications.show({
          color: 'red',
          title: 'Request failed',
          message: getRequestErrorMessage(error),
        });
        return false;
      } finally {
        setIsLoading(false);
      }
    },
    [],
  );

  return { execute, isLoading };
}
