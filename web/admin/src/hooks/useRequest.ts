import {useNotification} from "@kyvg/vue3-notification";

export function useRequest<R>(sendRequest: () => Promise<R>, onSuccess: (resp: R) => void): void {
    const notification = useNotification()

    sendRequest().then(onSuccess).catch((e: Error) => {
        console.error(e)
        notification.notify({
            type: 'error',
            text: e.message,
        })
    })
}
