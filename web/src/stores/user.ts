// 用户会话状态：accessToken 只存内存（刷新即丢，由路由守卫凭 refresh Cookie 恢复），
// user 持有 permCodes / roles / familyId —— 动态菜单、路由、按钮的唯一数据源。
import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import * as authApi from '@/api/auth'
import { refreshAccessToken } from '@/utils/request'
import type { LoginUser } from '@/types'

export const useUserStore = defineStore('user', () => {
  /** access token：仅内存，绝不写 localStorage（设计稿 §3.3） */
  const token = ref('')
  const user = ref<LoginUser | null>(null)
  /** 动态路由是否已注册（每次登录周期只 addRoute 一次） */
  const routesAdded = ref(false)

  const permCodes = computed(() => user.value?.permCodes ?? [])
  const roles = computed(() => user.value?.roles ?? [])

  /** 是否拥有某权限码 */
  function hasPerm(code: string): boolean {
    return permCodes.value.includes(code)
  }

  /** 是否持有任一给定角色 */
  function hasRole(...rs: string[]): boolean {
    return roles.value.some((r) => rs.includes(r))
  }

  /** 登录：存 token 与用户信息（refresh 已由后端写入 httpOnly Cookie） */
  async function login(username: string, password: string) {
    const data = await authApi.login(username, password)
    token.value = data.accessToken
    user.value = data.user
  }

  /**
   * 恢复会话：页面刷新后内存 token 丢失 —— 先凭 Cookie 里的 refresh
   * 主动换新 token（refreshAccessToken 内部已写回 store），再带 token 请求 /auth/me。
   * refresh 失败（无 Cookie / 过期 / 被撤销）会抛错，由路由守卫跳登录页。
   */
  async function fetchMe() {
    if (!token.value) {
      await refreshAccessToken()
    }
    user.value = await authApi.me()
  }

  /** 主动登出：调后端（jti 拉黑 + refresh 删除）后清空本地状态 */
  async function logout() {
    try {
      await authApi.logout()
    } catch {
      // 后端会话可能已过期，本地照常清理
    }
    clear()
  }

  /** 只清本地状态（会话被动失效 / 改密后强制下线） */
  function clear() {
    token.value = ''
    user.value = null
    routesAdded.value = false
  }

  return { token, user, routesAdded, permCodes, roles, hasPerm, hasRole, login, fetchMe, logout, clear }
})
