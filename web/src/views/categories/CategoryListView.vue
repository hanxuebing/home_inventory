<script setup lang="ts">
// 分类管理：家庭级分类；member 只读（无管理按钮），family_admin / admin 可增删改
import { onActivated, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listCategories, createCategory, updateCategory, deleteCategory } from '@/api/category'
import { useUserStore } from '@/stores/user'
import { fmtTime } from '@/utils/format'
import type { Category } from '@/types'

// 显式命名：KeepAlive include 按组件名匹配
defineOptions({ name: 'CategoryListView' })

const store = useUserStore()
const canManage = store.hasPerm('biz:category:create')

const list = ref<Category[]>([])
const loading = ref(true)

const dialog = reactive({ visible: false, isEdit: false, id: 0, name: '', sort: 0 })

async function load() {
  loading.value = true
  try {
    list.value = await listCategories()
  } finally {
    loading.value = false
  }
}

// KeepAlive 返回时刷新：分类可能在录入页被"快速新建"，或被其他成员改动
let firstActivation = true
onActivated(() => {
  if (firstActivation) {
    firstActivation = false // 首次挂载 onMounted 已加载
    return
  }
  load()
})

function openCreate() {
  Object.assign(dialog, { visible: true, isEdit: false, id: 0, name: '', sort: list.value.length + 1 })
}

function openEdit(c: Category) {
  Object.assign(dialog, { visible: true, isEdit: true, id: c.id, name: c.name, sort: c.sort })
}

async function save() {
  if (!dialog.name.trim()) {
    ElMessage.warning('请填写分类名')
    return
  }
  if (dialog.isEdit) {
    await updateCategory(dialog.id, dialog.name.trim(), dialog.sort)
    ElMessage.success('已保存')
  } else {
    await createCategory(dialog.name.trim(), dialog.sort)
    ElMessage.success('已创建')
  }
  dialog.visible = false
  load()
}

async function remove(c: Category) {
  await ElMessageBox.confirm(`删除分类「${c.name}」？该分类下的物品会转为"未分类"。`, '删除分类', {
    type: 'warning',
    confirmButtonText: '删除',
    confirmButtonClass: 'el-button--danger',
  })
  await deleteCategory(c.id)
  ElMessage.success('已删除')
  load()
}

onMounted(load)
</script>

<template>
  <div>
    <div class="cat-head">
      <span class="cat-tip">分类按家庭共享，录入物品时可选</span>
      <el-button v-if="canManage" type="primary" size="small" @click="openCreate">新建分类</el-button>
    </div>

    <div v-if="loading" class="page-loading"><el-spinner /></div>

    <div v-else-if="list.length" class="cat-list">
      <el-card v-for="c in list" :key="c.id" shadow="never" class="cat-card">
        <div class="cat-row">
          <div>
            <div class="cat-name">{{ c.name }}</div>
            <div class="cat-meta">排序 {{ c.sort }} · 建于 {{ fmtTime(c.createdAt) }}</div>
          </div>
          <div v-if="canManage" class="cat-ops">
            <el-button text type="primary" size="small" @click="openEdit(c)">编辑</el-button>
            <el-button text type="danger" size="small" @click="remove(c)">删除</el-button>
          </div>
        </div>
      </el-card>
    </div>

    <div v-else class="empty-tip">还没有分类，物品可以先不分类直接录入</div>

    <el-dialog v-model="dialog.visible" :title="dialog.isEdit ? '编辑分类' : '新建分类'" width="320px">
      <el-form label-position="top">
        <el-form-item label="分类名" required>
          <el-input v-model="dialog.name" maxlength="64" placeholder="如：厨房用品" />
        </el-form-item>
        <el-form-item label="排序（越小越靠前）">
          <el-input-number v-model="dialog.sort" :min="0" :max="999" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog.visible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.cat-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.cat-tip {
  font-size: 12px;
  color: #909399;
}

.cat-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.cat-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.cat-name {
  font-size: 15px;
  font-weight: 600;
}

.cat-meta {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
}

.cat-ops {
  flex-shrink: 0;
}
</style>
