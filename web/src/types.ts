// 全局类型定义 —— 与后端契约一一对应

/** 登录后的用户视图（/auth/login、/auth/me、/auth/refresh 返回） */
export interface LoginUser {
  id: number
  username: string
  nickname: string
  email: string | null
  familyId: number | null // admin 无家庭
  familyName: string | null
  roles: string[] // admin / family_admin / member
  permCodes: string[]
}

/** 家庭物品 */
export interface Item {
  id: number
  familyId: number
  name: string
  quantity: number
  categoryId: number | null
  categoryName: string | null
  image: string | null
  remark: string
  creatorId: number
  creatorName: string
  ownerId: number
  ownerName: string
  createdAt: string
  updatedAt: string
}

/** 物品修改历史动作：创建 / 修改 / 删除 / 责任移交 */
export type HistoryAction = 'CREATE' | 'UPDATE' | 'DELETE' | 'TRANSFER'

export interface ItemHistory {
  id: number
  itemId: number
  itemName: string
  operatorId: number
  operatorName: string
  action: HistoryAction
  before: Record<string, any> | null
  after: Record<string, any> | null
  createdAt: string
}

/** 分类（家庭级） */
export interface Category {
  id: number
  familyId: number
  name: string
  sort: number
  createdAt: string
}

/** 家庭（admin 视角，带统计） */
export interface Family {
  id: number
  name: string
  remark: string
  createdAt: string
  memberCount: number
  itemCount: number
}

/** 家庭成员（下拉/移交用） */
export interface FamilyMember {
  id: number
  username: string
  nickname: string
  roles: string[]
}

/** 管理端用户行 */
export interface UserRow {
  id: number
  username: string
  nickname: string
  email: string | null
  familyId: number | null
  familyName: string | null
  roleCodes: string[]
  status: number // 0禁用 1正常
  lastLoginAt: string | null
  createdAt: string
}

/** 审计日志行 */
export interface AuditLog {
  id: number
  userId: number | null
  username: string
  action: string
  target: string
  detail: string
  ip: string
  ua: string
  createdAt: string
}

/** 分页结构（后端统一） */
export interface Page<T> {
  list: T[]
  total: number
  page: number
  size: number
}
