import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { ApiError, parseError, unwrapApiData, type ApiBody } from './errors'

const http = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || 'http://localhost:8080/api/v1',
  timeout: 15000,
})

let refreshing: Promise<string> | null = null

function getRefreshToken() {
  return localStorage.getItem('refresh_token') || ''
}

function setTokens(access: string, refresh?: string) {
  localStorage.setItem('access_token', access)
  if (refresh) localStorage.setItem('refresh_token', refresh)
}

async function refreshAccessToken(): Promise<string> {
  const rt = getRefreshToken()
  if (!rt) throw new ApiError('登录已过期，请重新登录')
  const { data } = await axios.post(
    `${import.meta.env.VITE_API_BASE || 'http://localhost:8080/api/v1'}/auth/refresh`,
    { refresh_token: rt },
  )
  const body = data as ApiBody<{ tokens: { access_token: string; refresh_token?: string } }>
  const res = unwrapApiData(body)
  setTokens(res.tokens.access_token, res.tokens.refresh_token)
  window.dispatchEvent(new CustomEvent('squirtle:token-refreshed', { detail: res.tokens.access_token }))
  return res.tokens.access_token
}

http.interceptors.request.use((config) => {
  const token = localStorage.getItem('access_token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

http.interceptors.response.use(
  (res) => {
    const body = res.data as ApiBody
    if (body && typeof body.code === 'number' && body.code !== 0) {
      try {
        unwrapApiData(body)
      } catch (e) {
        return Promise.reject(e)
      }
    }
    return res
  },
  async (err: AxiosError) => {
    const original = err.config as InternalAxiosRequestConfig & { _retry?: boolean }
    const status = err.response?.status
    if (status === 401 && original && !original._retry && !original.url?.includes('/auth/refresh')) {
      original._retry = true
      try {
        refreshing = refreshing ?? refreshAccessToken()
        const token = await refreshing
        refreshing = null
        original.headers.Authorization = `Bearer ${token}`
        return http(original)
      } catch (e) {
        refreshing = null
        localStorage.removeItem('access_token')
        localStorage.removeItem('refresh_token')
        return Promise.reject(e instanceof ApiError ? e : new ApiError('登录已过期，请重新登录'))
      }
    }
    return Promise.reject(new ApiError(parseError(err)))
  },
)

export default http
export { unwrapApiData }
