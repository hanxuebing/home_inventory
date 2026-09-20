<script setup lang="ts">
// 403 / 404 兜底页
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const isNotFound = computed(() => Boolean(route.meta.notFound))

function back() {
  router.replace('/items')
}
</script>

<template>
  <div class="err-page">
    <div class="err-code">{{ isNotFound ? '404' : '403' }}</div>
    <div class="err-text">{{ isNotFound ? '页面不存在' : '没有权限访问该页面' }}</div>
    <el-button type="primary" plain @click="back">回首页</el-button>
  </div>
</template>

<style scoped>
.err-page {
  min-height: 100dvh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  background: #f5f7fa;
}

.err-code {
  font-size: 64px;
  font-weight: 800;
  color: #dcdfe6;
}

.err-text {
  font-size: 15px;
  color: #909399;
  margin-bottom: 8px;
}
</style>
