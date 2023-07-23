import {useEventSource} from "@vueuse/core";
import {watch} from "vue";
import config from "@/config";

abstract class EventHandler {
    handler: (data: any) => void

    constructor(handler: (data: any) => void) {
        this.handler = handler
    }

    abstract getName(): string

    abstract getData(data: string): any
}

export type EventConsumersData = { consumerCount: number, consumeTopicCount: number }

export class EventConsumers extends EventHandler {
    getName(): string {
        return 'customers';
    }

    getData(data: string): any {
        return JSON.parse(data) as EventConsumersData
    }
}

export type EventRepeaterData = { allCount: number, failedCount: number }

export class EventRepeater extends EventHandler {
    getName(): string {
        return 'repeater';
    }

    getData(data: string): any {
        return JSON.parse(data) as EventRepeaterData
    }
}

export function useServerEvent(eventHandlers: EventHandler[]): () => void {
    const listenEvents = eventHandlers.map(x => x.getName())
    const {event, data, close} = useEventSource(`${config.apiHost}/api/events`, listenEvents)
    watch(data, value => {
        if (value) {
            for (const h of eventHandlers) {
                if (h.getName() === event.value) {
                    h.handler(h.getData(value))
                }
            }
        }
    })
    return close
}
