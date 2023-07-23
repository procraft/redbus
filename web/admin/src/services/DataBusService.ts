import http from "@/http-common";

class DataBusService {
    getDashboardStat(): Promise<DashboardStatResponse> {
        return http.post("/dashboard/stat", '{}').then(x => x.data);
    }

    getRepeatStat(): Promise<RepeatStatResponse['list']> {
        return http.post("/repeat/stat", '{}').then(x => x.data.list);
    }

    repeatTopicGroup(topic: string, group: string): Promise<BasicResponse> {
        return http.post("/repeat/repeatTopicGroup", JSON.stringify({topic, group})).then(x => x.data);
    }
}

export default new DataBusService();

export type DashboardStatResponse = {
    consumerCount: number
    consumeTopicCount: number
    repeatAllCount: number
    repeatFailedCount: number
}

export type RepeatStatResponse = {
    list: Array<{
        topic: string,
        group: string,
        allCount: number,
        failedCount: number,
        lastError: string,
    }>
}

export type BasicResponse = {
    isSuccess: boolean,
    message?: string,
}
