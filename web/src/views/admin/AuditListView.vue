<script setup lang="ts">
// 审计日志（admin 全量 / family_admin 本家庭）：action 筛选 + 分页
import { onMounted, reactive, ref } from 'vue'
import { listAuditLogs } from '@/api/admin'
import { fmtTime, auditActions } from '@/utils/format'
import type { AuditLog } from '@/types'

const list = ref<AuditLog[]>([])
const total = ref(0)
const loading = ref(true)
const query = reactive({ action: '', page: 1, size: 10 })

/** action → 中文标签（未收录的显示原文） */
function label(a: string): string {
  return auditActions.find((x) => x.value === a)?.label ?? a
}

async function load() {
  loading.value = true
  try {
    const data = await listAuditLogs(query.action || undefined, query.page, query.size)
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function onPage(p: number) {
  query.page = p
  load()
}

onMounted(load)
</script>

<template>
  <div>
    <el-select
      v-model="query.action"
      placeholder="全部动作"
      clearable
      filterable
      style="width: 100%; margin-bottom: 12px"
      @change="load()"
    >
      <el-option v-for="a in auditActions" :key="a.value" :label="a.label" :value="a.value" />
    </el-select>

    <div v-if="loading" class="page-loading"><el-spinner /></div>

    <div v-else-if="list.length" class="log-list">
      <el-card v-for="l in list" :key="l.id" shadow="never">
        <div class="log-top">
          <el-tag size="small" :type="l.action.includes('FAIL') || l.action.includes('DELETE') ? 'danger' : l.action.includes('TRANSFER') ? 'warning' : 'info'">
            {{ label(l.action) }}
          </el-tag>
          <span class="log-user">{{ l.username }}</span>
          <span class="log-time">{{ fmtTime(l.createdAt) }}</span>
        </div>
        <div class="log-detail">
          <span v-if="l.target" class="log-target">对象：{{ l.target }}</span>
          <span>{{ l.detail || '-' }}</span>
        </div>
        <div class="log-ip">IP {{ l.ip }}</div>
      </el-card>
    </div>

    <div v-else class="empty-tip">暂无记录</div>

    <el-pagination
      v-if="total > query.size"
      layout="prev, pager, next"
      :total="total"
      :page-size="query.size"
      :current-page="query.page"
      small
      @current-change="onPage"
    />
  </div>
</template>

<style scoped>
.log-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.log-top {
  display: flex;
  align-items: center;
  gap: 8px;
}

.log-user {
  font-size: 13px;
  font-weight: 600;
}

.log-time {
  margin-left: auto;
  font-size: 12px;
  color: #c0c4cc;
}

.log-detail {
  margin-top: 6px;
  font-size: 13px;
  color: #606266;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.log-target {
  color: #909399;
}

.log-ip {
  margin-top: 4px;
  font-size: 11px;
  color: #c0c4cc;
}
</style>
