import axios, { AxiosInstance } from "axios";
import config from "@/config";

const apiClient: AxiosInstance = axios.create({
  baseURL: `${config.apiHost}/api`,
  headers: {
    "Content-type": "application/json",
  },
});

export default apiClient;
