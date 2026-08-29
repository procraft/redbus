import { useEffect } from 'react';

import type { ConsumersEvent, RepeaterEvent } from '@/api/types';
import config from '@/config';

type EventHandlers = {
  onConsumers: (data: ConsumersEvent) => void;
  onRepeater: (data: RepeaterEvent) => void;
  onOpen: () => void;
};

type LegacyRepeaterEvent = Omit<RepeaterEvent, 'failedCount'> & {
  failedCount?: number;
  failedount?: number;
};

export function useServerEvents({ onConsumers, onRepeater, onOpen }: EventHandlers): void {
  useEffect(() => {
    const eventSource = new EventSource(`${config.apiBaseUrl}/events`);

    eventSource.addEventListener('open', onOpen);

    const consumersHandler = (event: MessageEvent<string>) => {
      try {
        onConsumers(JSON.parse(event.data) as ConsumersEvent);
      } catch (error) {
        console.error('Cannot parse consumers event', error);
      }
    };

    const repeaterHandler = (event: MessageEvent<string>) => {
      try {
        const data = JSON.parse(event.data) as LegacyRepeaterEvent;
        onRepeater({
          allCount: data.allCount,
          failedCount: data.failedCount ?? data.failedount ?? 0,
        });
      } catch (error) {
        console.error('Cannot parse repeater event', error);
      }
    };

    eventSource.addEventListener('consumers', consumersHandler);
    eventSource.addEventListener('repeater', repeaterHandler);

    return () => {
      eventSource.removeEventListener('open', onOpen);
      eventSource.close();
    };
  }, [onConsumers, onOpen, onRepeater]);
}
