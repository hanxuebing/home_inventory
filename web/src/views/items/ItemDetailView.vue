<script setup lang="ts">
// 物品详情：全字段展示（提交人 creatorName / 当前责任人 ownerName）+ 修改历史时间线。
// 操作权限：owner 本人、家庭管理员、超管可编辑/删除（与后端 canManage 判定一致）
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getItem, getItemHistory, deleteItem } from '@/api/item'
import { useUserStore } from '@/stores/user'
import { fmtTime, actionText } from '@/utils/format'
import type { Item, ItemHistory } from '@/types'

const route = useRoute()
const router = useRouter()
const store = useUserStore()

const item = ref<Item | null>(null)
const history = ref<ItemHistory[]>([])
const loading = ref(true)

/** 编辑/删除权：本人负责 / 家庭管理员 / 超管 */
const canManage = computed(() => {
  const it = item.value
  if (!it) return false
  if (store.hasRole('admin')) return true
  if (store.hasRole('family_admin') && store.user?.familyId === it.familyId) return true
  return it.ownerId === store.user?.id
})

async function loadAll() {
  const id = route.params.id as string
  loading.value = true
  try {
    item.value = await getItem(id)
    history.value = await getItemHistory(id)
  } finally {
    loading.value = false
  }
}

async function onRemove() {
  const it = item.value!
  await ElMessageBox.confirm(`确定删除「${it.name}」吗？删除后历史记录仍会保留。`, '删除物品', {
    type: 'warning',
    confirmButtonText: '删除',
    confirmButtonClass: 'el-button--danger',
  })
  await deleteItem(it.id)
  ElMessage.success('已删除')
  router.replace('/items')
}

/** before/after 快照 → 简短差异描述 */
function diffText(h: ItemHistory): string {
  if (h.action === 'TRANSFER') {
    return `${h.before?.owner ?? '?'} → ${h.after?.owner ?? '?'}`
  }
  const parts: string[] = []
  const keys = ['name', 'quantity', 'image', 'remark']
  for (const k of keys) {
    const b = h.before?.[k]
    const a = h.after?.[k]
    if (String(b ?? '') !== String(a ?? '')) {
      if (h.action === 'CREATE') parts.push(`${k}: ${a ?? '空'}`)
      else parts.push(`${k}: ${b ?? '空'} → ${a ?? '空'}`)
    }
  }
  return parts.join('；') || '无字段变化'
}

onMounted(loadAll)
</script>

<template>
  <div>
    <div v-if="loading" class="page-loading"><el-spinner /></div>

    <template v-else-if="item">
      <!-- 大图 -->
      <div class="detail-img">
        <img v-if="item.image" :src="item.image" alt="" />
        <span v-else>📦</span>
      </div>

      <!-- 基本信息卡 -->
      <el-card shadow="never" class="detail-card">
        <div class="detail-head">
          <span class="detail-name">{{ item.name }}</span>
          <el-tag v-if="item.categoryName" type="info">{{ item.categoryName }}</el-tag>
          <el-tag v-else type="info" effect="plain">未分类</el-tag>
        </div>
        <div class="detail-qty">数量：{{ item.quantity }}</div>
        <div v-if="item.remark" class="detail-remark">{{ item.remark }}</div>

        <div class="detail-people">
          <div class="people-row"><span class="people-label">提交人</span>{{ item.creatorName }}</div>
          <div class="people-row">
            <span class="people-label">当前责任人</span>{{ item.ownerName }}
            <el-tag v-if="item.ownerId !== item.creatorId" size="small" type="warning" style="margin-left: 6px">
              已移交
            </el-tag>
          </div>
          <div class="people-row"><span class="people-label">更新时间</span>{{ fmtTime(item.updatedAt) }}</div>
        </div>

        <div v-if="canManage" class="detail-actions">
          <el-button type="primary" plain @click="router.push(`/items/${item.id}/edit`)">编辑</el-button>
          <el-button type="danger" plain @click="onRemove">删除</el-button>
        </div>
        <div v-else class="detail-actions-readonly">你只能查看该物品（由 {{ item.ownerName }} 负责）</div>
      </el-card>

      <!-- 修改历史时间线 -->
      <div class="section-title">修改历史</div>
      <el-card shadow="never">
        <el-timeline v-if="history.length">
          <el-timeline-item
            v-for="h in history"
            :key="h.id"
            :type="h.action === 'DELETE' ? 'danger' : h.action === 'TRANSFER' ? 'warning' : 'primary'"
            :timestamp="fmtTime(h.createdAt)"
          >
            <div class="hist-head">
              <b>{{ actionText[h.action] || h.action }}</b>
              <span class="hist-op">by {{ h.operatorName }}</span>
            </div>
            <div class="hist-diff">{{ diffText(h) }}</div>
          </el-timeline-item>
        </el-timeline>
        <div v-else class="empty-tip">暂无历史记录</div>
      </el-card>
    </template>
  </div>
</template>

<style scoped>
.detail-img {
  width: 100%;
  aspect-ratio: 4 / 3;
  max-height: 300px;
  border-radius: 12px;
  overflow: hidden;
  background: #f0f2f5;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 56px;
}

.detail-img img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  background: #fff;
}

.detail-card {
  margin-top: 12px;
}

.detail-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.detail-name {
  font-size: 18px;
  font-weight: 700;
}

.detail-qty {
  margin-top: 8px;
  font-size: 14px;
  color: #606266;
}

.detail-remark {
  margin-top: 8px;
  font-size: 13px;
  color: #909399;
  white-space: pre-wrap;
  word-break: break-all;
}

.detail-people {
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px dashed #e4e7ed;
  font-size: 13px;
  color: #606266;
}

.people-row {
  line-height: 2;
}

.people-label {
  display: inline-block;
  width: 84px;
  color: #909399;
}

.detail-actions {
  margin-top: 14px;
  display: flex;
  gap: 10px;
}

.detail-actions-readonly {
  margin-top: 14px;
  font-size: 12px;
  color: #c0c4cc;
}

.hist-head {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.hist-op {
  font-size: 12px;
  color: #909399;
}

.hist-diff {
  margin-top: 2px;
  font-size: 12px;
  color: #606266;
  word-break: break-all;
}
</style>
