// 通用格式化工具

/** ISO 时间 → YYYY-MM-DD HH:mm */
export function fmtTime(iso: string | null | undefined): string {
  if (!iso) return '-'
  const d = new Date(iso)
  if (isNaN(d.getTime())) return iso
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

/** 角色徽章文案与颜色（EP tag type） */
export function roleBadge(role: string): { text: string; type: 'danger' | 'warning' | 'primary' | 'info' } {
  switch (role) {
    case 'admin':
      return { text: '超级管理员', type: 'danger' }
    case 'family_admin':
      return { text: '家庭管理员', type: 'warning' }
    case 'member':
      return { text: '普通成员', type: 'primary' }
    default:
      return { text: role, type: 'info' }
  }
}

/** 历史动作 → 中文 */
export const actionText: Record<string, string> = {
  CREATE: '创建',
  UPDATE: '修改',
  DELETE: '删除',
  TRANSFER: '责任移交',
}

/** 审计动作 → 中文（下拉筛选用） */
export const auditActions: { value: string; label: string }[] = [
  { value: 'LOGIN_OK', label: '登录成功' },
  { value: 'LOGIN_FAIL', label: '登录失败' },
  { value: 'LOGIN_LOCKED', label: '登录锁定' },
  { value: 'LOGOUT', label: '登出' },
  { value: 'REUSE_DETECTED', label: '凭证重用告警' },
  { value: 'PASSWORD_CHANGE', label: '修改密码' },
  { value: 'USER_CREATE', label: '创建用户' },
  { value: 'USER_UPDATE', label: '更新用户' },
  { value: 'USER_DELETE', label: '删除用户' },
  { value: 'USER_ROLE', label: '设置角色' },
  { value: 'TRANSFER_ADMIN', label: '移交管理员' },
  { value: 'FAMILY_CREATE', label: '创建家庭' },
  { value: 'FAMILY_UPDATE', label: '更新家庭' },
  { value: 'FAMILY_DELETE', label: '删除家庭' },
  { value: 'ITEM_CREATE', label: '录入物品' },
  { value: 'ITEM_UPDATE', label: '编辑物品' },
  { value: 'ITEM_DELETE', label: '删除物品' },
  { value: 'CATEGORY_CREATE', label: '新建分类' },
  { value: 'CATEGORY_DELETE', label: '删除分类' },
]
