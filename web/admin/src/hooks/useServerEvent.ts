import {useEventSource} from "@vueuse/core";
import {watch} from "vue";

export function useServerEvent<T>(events: string[], handler: (event: string, data: T) => void): () => void {
    const { event, data, close } = useEventSource('http://localhost:50006/api/events', events)
    watch(data, value => {
        if (value) {
            const json = JSON.parse(value) as T
            handler(event.value || '', json)
        }
    })
    return close
}

export type EventConsumers = {consumerCount: number, consumeTopicCount: number}