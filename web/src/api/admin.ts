// 管理端 API（家庭 / 用户 / 审计）
import { http } from '@/utils/request'
import type { AuditLog, Family, FamilyMember, Page, UserRow } from '@/types'

// ---- 家庭 ----
export function listFamilies(): Promise<Family[]> {
  return http.get('/admin/families') as Promise<Family[]>
}

export function createFamily(name: string, remark = ''): Promise<{ id: number }> {
  return http.post('/admin/families', { name, remark }) as Promise<{ id: number }>
}

export function updateFamily(id: number | string, name: string, remark = ''): Promise<void> {
  return http.put(`/admin/families/${id}`, { name, remark }).then(() => undefined)
}

export function deleteFamily(id: number | string): Promise<void> {
  return http.delete(`/admin/families/${id}`).then(() => undefined)
}

export function familyMembers(familyId: number | string): Promise<FamilyMember[]> {
  return http.get(`/admin/families/${familyId}/members`) as Promise<FamilyMember[]>
}

/** 家庭管理员移交：target 成为家庭管理员，原管理员降为普通成员 */
export function transferFamilyAdmin(familyId: number | string, targetUserId: number): Promise<void> {
  return http.post(`/admin/families/${familyId}/transfer`, { targetUserId }).then(() => undefined)
}

// ---- 用户 ----
export interface UserQuery {
  familyId?: number | string
  keyword?: string
  status?: number | string
  page?: number
  size?: number
}

export function listUsers(q: UserQuery = {}): Promise<Page<UserRow>> {
  return http.get('/admin/users', { params: q }) as Promise<Page<UserRow>>
}

export interface CreateUserPayload {
  username: string
  password: string
  nickname: string
  email?: string
  familyId?: number | null
  roleCode?: string
}

export function createUser(p: CreateUserPayload): Promise<{ id: number }> {
  return http.post('/admin/users', p) as Promise<{ id: number }>
}

export function updateUser(
  id: number | string,
  p: { nickname: string; email?: string; status?: number },
): Promise<void> {
  return http.put(`/admin/users/${id}`, p).then(() => undefined)
}

export function setUserRole(id: number | string, roleCode: string, familyId?: number | null): Promise<void> {
  return http.put(`/admin/users/${id}/role`, { roleCode, familyId }).then(() => undefined)
}

/** 删除用户；名下有物品时必须带 receiverId（同家庭接收人），返回移交条数 */
export function deleteUser(id: number | string, receiverId?: number | null): Promise<{ transferred: number }> {
  return http.delete(`/admin/users/${id}`, { data: receiverId ? { receiverId } : {} }) as Promise<{
    transferred: number
  }>
}

// ---- 审计 ----
export function listAuditLogs(action?: string, page = 1, size = 10): Promise<Page<AuditLog>> {
  return http.get('/admin/audit-logs', { params: { action: action || undefined, page, size } }) as Promise<
    Page<AuditLog>
  >
}
