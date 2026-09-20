// 认证 API
import { http } from '@/utils/request'
import type { LoginUser } from '@/types'

export interface LoginResp {
  accessToken: string
  user: LoginUser
}

/** 登录（_skipRefresh：登录失败不该触发刷新链） */
export function login(username: string, password: string): Promise<LoginResp> {
  return http.post('/auth/login', { username, password }, { _skipRefresh: true } as any) as Promise<LoginResp>
}

/** 当前用户信息（无 token 时拦截器会自动走 refresh 再重放本请求） */
export function me(): Promise<LoginUser> {
  return http.get('/auth/me') as Promise<LoginUser>
}

/** 登出：后端把 jti 拉黑 + 删 refresh 白名单 */
export function logout(): Promise<void> {
  return http.post('/auth/logout', null, { _skipRefresh: true } as any).then(() => undefined)
}

/** 修改密码（成功后全部会话被撤销，需重新登录） */
export function changePassword(oldPassword: string, newPassword: string): Promise<void> {
  return http.put('/auth/password', { oldPassword, newPassword }).then(() => undefined)
}
