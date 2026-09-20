/**
 * axios 统一封装 —— 鉴权设计稿 §5 的"静默刷新"落地：
 *
 *  - accessToken 只在内存（Pinia），由请求拦截器注入 Bearer 头
 *  - refresh token 在 httpOnly Cookie 里，前端拿不到本体，只负责带上（withCredentials）
 *  - 任意请求收到 401：全局单飞调 /auth/refresh（并发 401 只发一次），
 *    成功后用新 access token 重放原请求；失败才清状态跳登录页
 *  - 403 / 400 弹 ElMessage 提示后端 message
 *
 * 成功响应直接 resolve 业务 data（拦截器已校验 code === 0）。
 */
import axios, { AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'

// 由 main.ts 绑定，避免 request ↔ store 循环依赖
let getToken: () => string = () => ''
let onAuthExpired: () => void = () => {}
let onNewToken: (token: string) => void = () => {}

export function bindAuth(get: () => string, expired: () => void, newToken: (t: string) => void) {
  getToken = get
  onAuthExpired = expired
  onNewToken = newToken
}

/** 业务错误（code != 0）统一形态 */
export class ApiError extends Error {
  code: string | number
  constructor(code: string | number, message: string) {
    super(message)
    this.code = code
  }
}

export const http = axios.create({
  baseURL: '/api/v1',
  withCredentials: true, // refresh 走 Cookie，跨端口开发也必须带凭据
  timeout: 20000,
})

http.interceptors.request.use((cfg) => {
  const t = getToken()
  if (t) cfg.headers.Authorization = `Bearer ${t}`
  return cfg
})

// ---- 全局单飞 refresh：并发 401 时共享同一个刷新 promise ----
let refreshing: Promise<string> | null = null

/**
 * 主动刷新（会话恢复用）：凭 httpOnly Cookie 换新 access token 并写回 store。
 * 页面刷新后 token 内存丢失，路由守卫先调这里、再带 token 请求 /auth/me ——
 * 避免"明知无 token 还硬发请求 → 401 → 拦截器补救"的无谓往返。
 */
export function refreshAccessToken(): Promise<string> {
  if (!refreshing) {
    refreshing = axios
      .post('/api/v1/auth/refresh', null, { withCredentials: true })
      .then((res) => {
        const d = res.data?.data
        if (res.data?.code !== 0 || !d?.accessToken) {
          throw new Error('刷新失败')
        }
        // 关键：写回 store！否则后续请求仍拿旧 token（或空），
        // 每个请求都要 401→refresh 一轮，还可能因频繁轮换触发重用检测误伤
        onNewToken(d.accessToken as string)
        return d.accessToken as string
      })
      .finally(() => {
        refreshing = null
      })
  }
  return refreshing
}

interface RetryConfig extends InternalAxiosRequestConfig {
  _retried?: boolean
  _skipRefresh?: boolean // login / logout 自身不参与刷新
}

http.interceptors.response.use(
  (res) => {
    // HTTP 200 但业务 code != 0：转为 reject，交给错误分支统一提示
    const body = res.data
    if (body && typeof body === 'object' && 'code' in body && body.code !== 0) {
      return Promise.reject(new ApiError(body.code, body.message || '请求失败'))
    }
    return body?.data // 成功：直接把业务 data 交给调用方
  },
  async (err: AxiosError<any>) => {
    const cfg = err.config as RetryConfig | undefined
    const status = err.response?.status

    // ---- 401：静默刷新并重放 ----
    if (status === 401 && cfg && !cfg._retried && !cfg._skipRefresh) {
      cfg._retried = true
      try {
        const newToken = await refreshAccessToken()
        cfg.headers.Authorization = `Bearer ${newToken}`
        return http.request(cfg)
      } catch {
        onAuthExpired() // refresh 也失效：清状态 → 跳登录页
        return Promise.reject(new ApiError('A0231', '登录已过期，请重新登录'))
      }
    }

    // ---- 401（已重试/不可刷新）----
    if (status === 401) {
      onAuthExpired()
      return Promise.reject(new ApiError('A0231', '请先登录'))
    }

    // ---- 其他错误：取后端 message 提示 ----
    const msg: string =
      (err.response?.data as any)?.message || (err instanceof ApiError ? err.message : '网络异常，请稍后重试')
    if (err.response?.config && status !== 403) {
      // 403 通常由页面级 UI 呈现（隐藏按钮），这里统一轻提示
    }
    ElMessage.error(msg)
    return Promise.reject(new ApiError((err.response?.data as any)?.code ?? -1, msg))
  },
)
