<script setup lang="ts">
// 家庭管理（仅 admin）：家庭卡片 + 创建/编辑/删除 + 查看成员
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listFamilies, createFamily, updateFamily, deleteFamily } from '@/api/admin'
import type { Family } from '@/types'

const router = useRouter()
const list = ref<Family[]>([])
const loading = ref(true)

const dialog = reactive({ visible: false, isEdit: false, id: 0, name: '', remark: '' })

async function load() {
  loading.value = true
  try {
    list.value = await listFamilies()
  } finally {
    loading.value = false
  }
}

function openCreate() {
  Object.assign(dialog, { visible: true, isEdit: false, id: 0, name: '', remark: '' })
}

function openEdit(f: Family) {
  Object.assign(dialog, { visible: true, isEdit: true, id: f.id, name: f.name, remark: f.remark })
}

async function save() {
  if (!dialog.name.trim()) {
    ElMessage.warning('请填写家庭名')
    return
  }
  if (dialog.isEdit) {
    await updateFamily(dialog.id, dialog.name.trim(), dialog.remark)
    ElMessage.success('已保存')
  } else {
    await createFamily(dialog.name.trim(), dialog.remark)
    ElMessage.success('已创建')
  }
  dialog.visible = false
  load()
}

async function remove(f: Family) {
  await ElMessageBox.confirm(
    `删除家庭「${f.name}」？家庭内还有成员时无法删除（需先处理成员）。`,
    '删除家庭',
    { type: 'warning', confirmButtonText: '删除', confirmButtonClass: 'el-button--danger' },
  )
  try {
    await deleteFamily(f.id)
    ElMessage.success('已删除')
    load()
  } catch {
    // "家庭内还有成员" 的后端提示由拦截器弹出
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="fam-head">
      <span class="fam-title">全部家庭</span>
      <el-button type="primary" size="small" @click="openCreate">创建家庭</el-button>
    </div>

    <div v-if="loading" class="page-loading"><el-spinner /></div>

    <div v-else-if="list.length" class="fam-list">
      <el-card v-for="f in list" :key="f.id" shadow="never">
        <div class="fam-row" @click="router.push({ path: '/admin/members', query: { familyId: f.id } })">
          <div class="fam-info">
            <div class="fam-name">🏠 {{ f.name }}</div>
            <div class="fam-meta">
              {{ f.remark || '暂无备注' }} · 成员 {{ f.memberCount }} · 物品 {{ f.itemCount }}
            </div>
          </div>
          <div class="fam-ops" @click.stop>
            <el-button text type="primary" size="small" @click="openEdit(f)">编辑</el-button>
            <el-button text type="danger" size="small" @click="remove(f)">删除</el-button>
          </div>
        </div>
      </el-card>
    </div>

    <div v-else class="empty-tip">还没有家庭，先创建一个</div>

    <el-dialog v-model="dialog.visible" :title="dialog.isEdit ? '编辑家庭' : '创建家庭'" width="320px">
      <el-form label-position="top" @submit.prevent="save">
        <el-form-item label="家庭名" required>
          <el-input v-model="dialog.name" maxlength="64" placeholder="如：张家" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="dialog.remark" maxlength="255" placeholder="可选" />
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
.fam-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.fam-title {
  font-size: 14px;
  font-weight: 600;
}

.fam-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.fam-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.fam-name {
  font-size: 15px;
  font-weight: 600;
}

.fam-meta {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
}

.fam-ops {
  flex-shrink: 0;
}
</style>
