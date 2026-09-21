// 路由：静态路由（登录/错误页）+ 动态路由（按 permCodes 过滤后注册）+ 全局守卫
// 守卫流程（设计稿图 5）：白名单直通 → 无会话先尝试凭 refresh Cookie 恢复 →
// 失败跳登录 → 注册动态路由 → 逐路由校验 meta.permission，无权跳 403
import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useUserStore } from '@/stores/user'

// 静态路由
const staticRoutes: RouteRecordRaw[] = [
  { path: '/login', name: 'login', component: () => import('@/views/login/LoginView.vue'), meta: { public: true } },
  { path: '/403', name: 'forbidden', component: () => import('@/views/error/ForbiddenView.vue'), meta: { public: true } },
  { path: '/:pathMatch(.*)*', name: 'not-found', component: () => import('@/views/error/ForbiddenView.vue'), meta: { public: true, notFound: true } },
]

const router = createRouter({
  history: createWebHistory(),
  routes: staticRoutes,
})

// 动态路由表：meta.permission 是该页面的准入权限码
const dynamicRoutes: RouteRecordRaw[] = [
  {
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    redirect: '/items',
    children: [
      { path: 'items', name: 'items', component: () => import('@/views/items/ItemListView.vue'), meta: { permission: 'biz:item:page', tab: 'items', keepAlive: true, title: '家庭物品' } },
      { path: 'items/new', name: 'item-new', component: () => import('@/views/items/ItemEditView.vue'), meta: { permission: 'biz:item:create', title: '录入物品' } },
      { path: 'items/:id', name: 'item-detail', component: () => import('@/views/items/ItemDetailView.vue'), meta: { permission: 'biz:item:list', title: '物品详情' } },
      { path: 'items/:id/edit', name: 'item-edit', component: () => import('@/views/items/ItemEditView.vue'), meta: { permission: 'biz:item:update', title: '编辑物品' } },
      { path: 'categories', name: 'categories', component: () => import('@/views/categories/CategoryListView.vue'), meta: { permission: 'biz:category:page', tab: 'categories', keepAlive: true, title: '分类管理' } },
      { path: 'profile', name: 'profile', component: () => import('@/views/profile/ProfileView.vue'), meta: { tab: 'profile', keepAlive: true, title: '我的' } },

      // ---- 管理功能：按权限动态出现（family_admin 可见成员管理；admin 全见）----
      // tab: 'profile' —— 都是从"我的"页进入的，底部 tabbar 高亮"我的"
      { path: 'admin/members', name: 'admin-members', component: () => import('@/views/admin/MemberListView.vue'), meta: { permission: 'sys:user:page', tab: 'profile', title: '成员管理' } },
      { path: 'admin/families', name: 'admin-families', component: () => import('@/views/admin/FamilyListView.vue'), meta: { permission: 'sys:family:page', tab: 'profile', title: '家庭管理' } },
      { path: 'admin/users', name: 'admin-users', component: () => import('@/views/admin/UserListView.vue'), meta: { permission: 'sys:family:page', tab: 'profile', title: '用户管理' } },
      { path: 'admin/audit', name: 'admin-audit', component: () => import('@/views/admin/AuditListView.vue'), meta: { permission: 'sys:audit:page', tab: 'profile', title: '审计日志' } },
    ],
  },
]

// 白名单：无需登录即可访问
function isPublic(path: string): boolean {
  return path === '/login' || path === '/403'
}

// filterAsyncRoutes（设计稿 §5）：按权限码过滤后再注册，
// 无权页面的组件根本不进入路由表 —— 既省下载，也避免暴露无权页面的存在
function filterRoutes(routes: RouteRecordRaw[], hasPerm: (code: string) => boolean): RouteRecordRaw[] {
  const out: RouteRecordRaw[] = []
  for (const r of routes) {
    const perm = r.meta?.permission as string | undefined
    if (perm && !hasPerm(perm)) continue // 无权限码要求的路由（如"我的"）直接保留
    if (r.children?.length) {
      out.push({ ...r, children: filterRoutes(r.children, hasPerm) })
    } else {
      out.push({ ...r })
    }
  }
  return out
}

router.beforeEach(async (to) => {
  if (isPublic(to.path)) return true

  const store = useUserStore()

  // 尚无用户信息（首次进入或刚刷新）：尝试凭 refresh Cookie 静默恢复会话
  if (!store.user) {
    try {
      await store.fetchMe()
    } catch {
      return { name: 'login', query: { redirect: to.fullPath } }
    }
  }

  // 注册过滤后的动态路由（一次），注册后 replace 重进以命中新路由。
  // 注意用 path 而不是展开 to —— to 是注册前解析的（此时 /items 还不存在，
  // 匹配到的是 catch-all，name='not-found'），展开会带上这个 name 导致永远跳 404
  if (!store.routesAdded) {
    for (const route of filterRoutes(dynamicRoutes, (c) => store.hasPerm(c))) {
      router.addRoute(route)
    }
    store.routesAdded = true
    return { path: to.fullPath, replace: true }
  }

  // 路由级权限校验（双保险：即使路由表被篡改也拦得住）
  const perm = to.meta.permission as string | undefined
  if (perm && !store.hasPerm(perm)) {
    return { name: 'forbidden' }
  }
  return true
})

export default router
