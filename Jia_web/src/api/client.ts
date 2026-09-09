import axios, { type AxiosRequestConfig } from 'axios'

export const apiBaseURL = import.meta.env.VITE_API_URL || `http://${typeof window === 'undefined' ? 'localhost' : window.location.hostname}:8081/api/v1`
export const apiOrigin = apiBaseURL.replace(/\/api\/v1\/?$/, '')
const client = axios.create({ baseURL: apiBaseURL })
client.interceptors.request.use((config) => {
  const token = localStorage.getItem('jia_token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})
client.interceptors.response.use((response) => response.data?.data ?? response.data, (error) => {
		const message = error.response?.data?.error?.message || (error.response ? `请求失败（${error.response.status}）` : `无法连接 Jia API：${apiBaseURL}`)
	return Promise.reject(new Error(message))
})

export const api = {
  get: <T>(url: string, config?: AxiosRequestConfig) => client.get<T, T>(url, config),
  post: <T>(url: string, data?: unknown, config?: AxiosRequestConfig) => client.post<T, T>(url, data, config),
  patch: <T>(url: string, data?: unknown, config?: AxiosRequestConfig) => client.patch<T, T>(url, data, config),
  delete: <T>(url: string, config?: AxiosRequestConfig) => client.delete<T, T>(url, config),
}
