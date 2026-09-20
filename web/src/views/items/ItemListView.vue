<script setup lang="ts">
// 物品列表：模糊搜索（名称/备注）+ 分类筛选 + 家庭共享列表 + 加载更多分页
// 每张卡片显示提交人与当前责任人；member 对非本人负责的物品只读（详情页控制操作）
import { onActivated, onDeactivated, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Search, Plus } from '@element-plus/icons-vue'
import { listItems } from '@/api/item'
import { listCategories } from '@/api/category'
import type { Category, Item } from '@/types'

// 显式命名：KeepAlive include 按组件名匹配（MainLayout 里 include="ItemListView"）
defineOptions({ name: 'ItemListView' })

const router = useRouter()

const keyword = ref('')
const categoryId = ref<string>('')
const categories = ref<Category[]>([])

const items = ref<Item[]>([])
const total = ref(0)
const page = reactive({ no: 1, size: 20 })
const loading = ref(false)
let searchTimer: ReturnType<typeof setTimeout> | null = null

async function load(reset = false) {
  loading.value = true
  try {
    if (reset) page.no = 1
    const data = await listItems({
      keyword: keyword.value.trim() || undefined,
      categoryId: categoryId.value || undefined,
      page: page.no,
      size: page.size,
    })
    items.value = reset ? data.list : [...items.value, ...data.list]
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function loadMore() {
  page.no++
  load()
}

/** 搜索防抖：输入停顿 400ms 再发请求 */
function onKeyword() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => load(true), 400)
}

function onCategory(v: string) {
  categoryId.value = v
  load(true)
}

onMounted(async () => {
  load(true)
  try {
    categories.value = await listCategories()
  } catch {
    // 分类加载失败不阻塞列表
  }
})

// ---- KeepAlive 状态保留：搜索词 / 分类筛选 / 已加载页数 / 滚动位置 ----
// 离开时记录滚动位置；返回时按"已加载的条数"整段刷新（保留筛选与页数，
// 新建/编辑过的物品也能立刻看到），然后恢复滚动位置。
let savedScroll = 0
let firstActivation = true

onDeactivated(() => {
  savedScroll = window.scrollY
})

onActivated(async () => {
  if (firstActivation) {
    firstActivation = false // 首次挂载 onMounted 已加载，跳过
    return
  }
  loading.value = true
  try {
    // 一次拉回已加载的全部条数（page:1 + size=已加载页数*每页），替换而非追加
    const data = await listItems({
      keyword: keyword.value.trim() || undefined,
      categoryId: categoryId.value || undefined,
      page: 1,
      size: Math.min(page.no * page.size, 100), // 后端 size 上限 100
    })
    items.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
  window.scrollTo(0, savedScroll)
})
</script>

<template>
  <div>
    <!-- 搜索框 -->
    <el-input
      v-model="keyword"
      placeholder="搜索物品名称 / 备注"
      clearable
      size="large"
      :prefix-icon="Search"
      @input="onKeyword"
      @clear="load(true)"
    />

    <!-- 分类横向筛选 chips -->
    <div class="cat-chips">
      <button
        v-for="opt in [{ id: '', name: '全部' }, { id: 'none', name: '未分类' }, ...categories.map((c) => ({ id: String(c.id), name: c.name }))]"
        :key="opt.id"
        class="chip"
        :class="{ active: categoryId === opt.id }"
        @click="onCategory(opt.id)"
      >
        {{ opt.name }}
      </button>
    </div>

    <!-- 物品卡片列表 -->
    <div v-if="items.length" class="item-list">
      <el-card v-for="it in items" :key="it.id" shadow="never" class="item-card" @click="router.push(`/items/${it.id}`)">
        <div class="item-row">
          <div class="item-thumb">
            <img v-if="it.image" :src="it.image" alt="" loading="lazy" />
            <span v-else class="thumb-placeholder">📦</span>
          </div>
          <div class="item-info">
            <div class="item-name-row">
              <span class="item-name">{{ it.name }}</span>
              <el-tag v-if="it.categoryName" size="small" type="info">{{ it.categoryName }}</el-tag>
            </div>
            <div class="item-meta">数量 {{ it.quantity }} · 提交人 {{ it.creatorName }}</div>
            <div v-if="it.ownerId !== it.creatorId" class="item-meta owner">现由 {{ it.ownerName }} 负责</div>
          </div>
        </div>
      </el-card>
    </div>

    <div v-else-if="!loading" class="empty-tip">还没有物品，点右下角 + 录入第一件吧</div>

    <div v-if="loading" class="page-loading"><el-spinner /></div>
    <div v-else-if="items.length && items.length < total" class="load-more">
      <el-button text type="primary" @click="loadMore">加载更多（{{ items.length }}/{{ total }}）</el-button>
    </div>

    <!-- 浮动录入按钮：无录入权限的账号不会看到 -->
    <button v-permission="'biz:item:create'" class="fab" @click="router.push('/items/new')">
      <el-icon :size="24"><Plus /></el-icon>
    </button>
  </div>
</template>

<style scoped>
.cat-chips {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding: 10px 0 4px;
  -webkit-overflow-scrolling: touch;
}

.cat-chips::-webkit-scrollbar {
  display: none;
}

.chip {
  flex-shrink: 0;
  min-height: 30px;
  padding: 0 14px;
  border-radius: 999px;
  border: 1px solid #dcdfe6;
  background: #fff;
  color: #606266;
  font-size: 13px;
  cursor: pointer;
}

.chip.active {
  border-color: var(--el-color-primary);
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}

.item-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 6px;
}

.item-card {
  cursor: pointer;
}

.item-row {
  display: flex;
  gap: 12px;
}

.item-thumb {
  width: 80px;
  height: 80px;
  flex-shrink: 0;
  border-radius: 8px;
  overflow: hidden;
  background: #f0f2f5;
  display: flex;
  align-items: center;
  justify-content: center;
}

.item-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.thumb-placeholder {
  font-size: 30px;
}

.item-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 4px;
}

.item-name-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.item-name {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item-meta {
  font-size: 12px;
  color: #909399;
}

.item-meta.owner {
  color: var(--el-color-warning);
}

.load-more {
  text-align: center;
  padding: 6px 0 10px;
}

.fab {
  position: fixed;
  right: calc(50% - 240px + 18px);
  bottom: calc(76px + env(safe-area-inset-bottom, 0px));
  width: 54px;
  height: 54px;
  border-radius: 50%;
  border: none;
  background: var(--el-color-primary);
  color: #fff;
  box-shadow: 0 4px 14px rgba(64, 158, 255, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  z-index: 9;
}

/* 窄屏（>480px 视口减半时 fab 贴右缘） */
@media (max-width: 508px) {
  .fab {
    right: 18px;
  }
}
</style>
