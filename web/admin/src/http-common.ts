import axios, { AxiosInstance } from "axios";

const apiClient: AxiosInstance = axios.create({
  baseURL: "http://localhost:50006/api",
  headers: {
    "Content-type": "application/json",
  },
});

export default apiClient;
