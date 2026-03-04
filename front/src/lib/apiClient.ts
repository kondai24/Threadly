import Axios from "axios";
import type { AxiosRequestConfig } from "axios";

export const api = Axios.create({
  baseURL: "",
});

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response == null) {
      return Promise.reject(new Error("レスポンスがありません。"));
    }
    const message =
      error.response.data?.message || `Error: ${error.response.status}`;
    return Promise.reject(new Error(message));
  },
);

export const customInstance = <T>(
  url: string,
  config: RequestInit,
): Promise<T> => {
  const source = Axios.CancelToken.source();

  const axiosConfig: AxiosRequestConfig = {
    url,
    method: config.method as AxiosRequestConfig["method"],
    cancelToken: source.token,
  };

  if (config.body) {
    axiosConfig.data =
      typeof config.body === "string" ? JSON.parse(config.body) : config.body;
  }

  if (config.headers) {
    const headers: Record<string, string> = {};
    if (config.headers instanceof Headers) {
      config.headers.forEach((value, key) => {
        headers[key] = value;
      });
    } else if (typeof config.headers === "object") {
      Object.assign(headers, config.headers);
    }
    axiosConfig.headers = headers;
  }

  if (config.signal) {
    axiosConfig.signal = config.signal as AbortSignal;
  }

  const promise = api(axiosConfig).then(({ data }) => data);
  return promise;
};
