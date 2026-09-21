<script setup lang="ts">
// 主布局：顶部标题栏 + 内容区 + 底部 tabbar（移动端三 tab：物品 / 分类 / 我的）
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Box, Files, User } from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()

const active = computed(() => (route.meta.tab as string) || 'items')
const title = computed(() => (route.meta.title as string) || '家庭物品')

const tabs = [
  { key: 'items', label: '物品', icon: Box, to: '/items' },
  { key: 'categories', label: '分类', icon: Files, to: '/categories' },
  { key: 'profile', label: '我的', icon: User, to: '/profile' },
]

function go(to: string) {
  router.push(to)
}
</script>

<template>
  <div class="layout">
    <!-- 顶部标题栏 -->
    <header class="layout-header">
      <span class="layout-title">{{ title }}</span>
    </header>

    <!-- 内容区：max-width 480px 居中，底部留出 tabbar 高度 -->
    <!-- 缓存策略（vue-router 官方推荐模式）：meta.keepAlive 的路由进 KeepAlive，
         其余（编辑/详情带 :id、管理页）走非缓存分支。
         不用 KeepAlive include —— 按组件名匹配在懒加载路由下不可靠 -->
    <main class="layout-main">
      <div class="layout-container">
        <router-view v-slot="{ Component, route }">
          <!-- 注意：注释不能写在 KeepAlive 里面 —— 注释也是子节点，
               会触发 "expects exactly one child component" 编译错误。
               key 用路由名：tab 页各占一个稳定缓存实例 -->
          <KeepAlive>
            <component :is="Component" v-if="route.meta.keepAlive" :key="route.name" />
          </KeepAlive>
          <!-- 非缓存页 key 用完整 path：详情 /items/3 → /items/5 是两个实例，数据各自加载 -->
          <component :is="Component" v-if="!route.meta.keepAlive" :key="route.path" />
        </router-view>
      </div>
    </main>

    <!-- 底部 tabbar：固定三个入口，管理功能在"我的"页按权限出现 -->
    <nav class="layout-tabbar">
      <button
        v-for="t in tabs"
        :key="t.key"
        class="tabbar-item"
        :class="{ active: active === t.key }"
        @click="go(t.to)"
      >
        <el-icon :size="22"><component :is="t.icon" /></el-icon>
        <span>{{ t.label }}</span>
      </button>
    </nav>
  </div>
</template>

<style scoped>
.layout {
  min-height: 100dvh;
  display: flex;
  flex-direction: column;
}

.layout-header {
  position: sticky;
  top: 0;
  z-index: 10;
  height: 48px;
  background: #fff;
  border-bottom: 1px solid #e4e7ed;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 16px;
  padding-top: env(safe-area-inset-top, 0px);
}

.layout-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.layout-main {
  flex: 1;
  width: 100%;
  max-width: 480px;
  margin: 0 auto;
  padding: 12px 14px calc(72px + env(safe-area-inset-bottom, 0px));
  box-sizing: border-box;
}

.layout-tabbar {
  position: fixed;
  left: 50%;
  transform: translateX(-50%);
  bottom: 0;
  width: 100%;
  max-width: 480px;
  display: flex;
  background: #fff;
  border-top: 1px solid #e4e7ed;
  padding-bottom: env(safe-area-inset-bottom, 0px);
  z-index: 10;
}

.tabbar-item {
  flex: 1;
  min-height: 52px;
  border: none;
  background: none;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  font-size: 11px;
  color: #909399;
  cursor: pointer;
}

.tabbar-item.active {
  color: var(--el-color-primary);
}
</style>
