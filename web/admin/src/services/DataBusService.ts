import http from "@/http-common";

class DataBusService {
  getDashboardStat(): Promise<DashboardStatResponse> {
    return http.post("/dashboard/stat", '{}').then(x => x.data);
  }
}

export default new DataBusService();

export type DashboardStatResponse = {
  consumerCount: number
  consumeTopicCount: number
  repeatAllCount: number
  repeatFailedCount: number
}