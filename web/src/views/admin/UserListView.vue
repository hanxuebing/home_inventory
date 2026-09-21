<script setup lang="ts">
// 用户管理（仅 admin）：全部用户、家庭/关键词/状态筛选、创建（含角色与家庭）、
// 设置角色、编辑、删除（接收人可选，输入昵称确认）
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import {
  listUsers, createUser, updateUser, setUserRole, deleteUser, listFamilies, familyMembers,
} from '@/api/admin'
import { roleBadge, fmtTime } from '@/utils/format'
import type { Family, UserRow } from '@/types'

const list = ref<UserRow[]>([])
const families = ref<Family[]>([])
const total = ref(0)
const loading = ref(true)

const query = reactive({ keyword: '', familyId: '' as string | number, status: '' as string | number, page: 1, size: 10 })

// ---- 创建 ----
const createDialog = reactive({
  visible: false, username: '', password: '', nickname: '', email: '',
  roleCode: 'member', familyId: null as number | null, saving: false,
})

// ---- 编辑 ----
const editDialog = reactive({ visible: false, row: null as UserRow | null, nickname: '', email: '', status: 1, saving: false })

// ---- 设置角色 ----
const roleDialog = reactive({ visible: false, row: null as UserRow | null, roleCode: 'member', familyId: null as number | null, saving: false })

// ---- 删除（接收人可选 + 输入昵称确认） ----
const delDialog = reactive({
  visible: false, row: null as UserRow | null, receiverId: null as number | null,
  confirmName: '', saving: false, receiverOptions: [] as { id: number; nickname: string }[],
})

async function load() {
  loading.value = true
  try {
    const data = await listUsers({
      keyword: query.keyword.trim() || undefined,
      familyId: query.familyId || undefined,
      status: query.status === '' ? undefined : query.status,
      page: query.page,
      size: query.size,
    })
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function openCreate() {
  Object.assign(createDialog, {
    visible: true, username: '', password: '', nickname: '', email: '',
    roleCode: 'member', familyId: families.value[0]?.id ?? null,
  })
}

async function saveCreate() {
  if (createDialog.password.length < 10) {
    ElMessage.warning('初始密码至少 10 位')
    return
  }
  if (createDialog.roleCode !== 'admin' && !createDialog.familyId) {
    ElMessage.warning('非超管角色必须归属一个家庭')
    return
  }
  createDialog.saving = true
  try {
    await createUser({
      username: createDialog.username.trim(),
      password: createDialog.password,
      nickname: createDialog.nickname.trim(),
      email: createDialog.email || undefined,
      familyId: createDialog.roleCode === 'admin' ? null : createDialog.familyId,
      roleCode: createDialog.roleCode,
    })
    ElMessage.success('用户已创建')
    createDialog.visible = false
    load()
  } catch {
  } finally {
    createDialog.saving = false
  }
}

function openEdit(row: UserRow) {
  Object.assign(editDialog, { visible: true, row, nickname: row.nickname, email: row.email ?? '', status: row.status })
}

async function saveEdit() {
  editDialog.saving = true
  try {
    await updateUser(editDialog.row!.id, {
      nickname: editDialog.nickname,
      email: editDialog.email || undefined,
      status: editDialog.status,
    })
    ElMessage.success('已保存')
    editDialog.visible = false
    load()
  } catch {
  } finally {
    editDialog.saving = false
  }
}

function openRole(row: UserRow) {
  Object.assign(roleDialog, {
    visible: true, row,
    roleCode: row.roleCodes[0] ?? 'member',
    familyId: row.familyId,
  })
}

async function saveRole() {
  if (roleDialog.roleCode !== 'admin' && !roleDialog.familyId) {
    ElMessage.warning('非超管角色必须归属一个家庭')
    return
  }
  roleDialog.saving = true
  try {
    await setUserRole(roleDialog.row!.id, roleDialog.roleCode, roleDialog.roleCode === 'admin' ? null : roleDialog.familyId)
    ElMessage.success('角色已更新，该用户需重新登录')
    roleDialog.visible = false
    load()
  } catch {
  } finally {
    roleDialog.saving = false
  }
}

async function openDelete(row: UserRow) {
  Object.assign(delDialog, { visible: true, row, receiverId: null, confirmName: '', receiverOptions: [] })
  // 有家庭才拉接收人候选（admin 用户无家庭）
  if (row.familyId) {
    try {
      const members = await familyMembers(row.familyId)
      delDialog.receiverOptions = members.filter((m) => m.id !== row.id).map((m) => ({ id: m.id, nickname: m.nickname }))
    } catch {
      // 忽略：不选接收人时物品将随成员一并删除
    }
  }
}

async function confirmDelete() {
  const row = delDialog.row!
  delDialog.saving = true
  try {
    const { transferred, purged } = await deleteUser(row.id, {
      receiverId: delDialog.receiverId,
      confirmName: delDialog.confirmName.trim(),
    })
    ElMessage.success(
      transferred > 0
        ? `已删除，${transferred} 件物品责任已移交`
        : purged > 0
          ? `已删除，名下 ${purged} 件物品一并删除`
          : '已删除',
    )
    delDialog.visible = false
    load()
  } catch {
    // 确认昵称不匹配等提示由拦截器弹出
  } finally {
    delDialog.saving = false
  }
}

function onPage(p: number) {
  query.page = p
  load()
}

onMounted(async () => {
  load()
  families.value = await listFamilies()
})
</script>

<template>
  <div>
    <!-- 筛选 -->
    <div class="filter-bar">
      <el-input v-model="query.keyword" placeholder="账号 / 昵称" clearable :prefix-icon="Search" @input="load()" @clear="load()" />
      <el-select v-model="query.familyId" placeholder="全部家庭" clearable @change="load()">
        <el-option v-for="f in families" :key="f.id" :label="f.name" :value="f.id" />
      </el-select>
      <el-select v-model="query.status" placeholder="状态" clearable style="width: 100px" @change="load()">
        <el-option label="正常" :value="1" />
        <el-option label="禁用" :value="0" />
      </el-select>
    </div>

    <el-button type="primary" style="width: 100%; margin-bottom: 12px" @click="openCreate">创建用户</el-button>

    <div v-if="loading" class="page-loading"><el-spinner /></div>

    <div v-else class="user-list">
      <el-card v-for="u in list" :key="u.id" shadow="never">
        <div class="user-top">
          <div class="user-name">
            {{ u.nickname }}
            <span class="user-username">@{{ u.username }}</span>
          </div>
          <el-tag :type="u.status === 1 ? 'success' : 'info'" size="small">
            {{ u.status === 1 ? '正常' : '禁用' }}
          </el-tag>
        </div>
        <div class="user-meta">
          <el-tag v-for="r in u.roleCodes" :key="r" :type="roleBadge(r).type" size="small">
            {{ roleBadge(r).text }}
          </el-tag>
          <el-tag v-if="u.familyName" type="success" size="small" effect="plain">{{ u.familyName }}</el-tag>
        </div>
        <div class="user-sub">最近登录 {{ fmtTime(u.lastLoginAt) }}</div>
        <div class="user-ops">
          <el-button text type="primary" size="small" @click="openEdit(u)">编辑</el-button>
          <el-button text type="warning" size="small" @click="openRole(u)">角色</el-button>
          <el-button v-if="!u.roleCodes.includes('admin')" text type="danger" size="small" @click="openDelete(u)">
            删除
          </el-button>
        </div>
      </el-card>
    </div>

    <el-pagination
      v-if="total > query.size"
      layout="prev, pager, next"
      :total="total"
      :page-size="query.size"
      :current-page="query.page"
      small
      @current-change="onPage"
    />

    <!-- 创建用户 -->
    <el-dialog v-model="createDialog.visible" title="创建用户" width="340px">
      <el-form label-position="top" @submit.prevent="saveCreate">
        <el-form-item label="账号" required><el-input v-model="createDialog.username" /></el-form-item>
        <el-form-item label="初始密码（>=10 位）" required>
          <el-input v-model="createDialog.password" type="password" show-password />
        </el-form-item>
        <el-form-item label="昵称" required><el-input v-model="createDialog.nickname" /></el-form-item>
        <el-form-item label="邮箱"><el-input v-model="createDialog.email" placeholder="可选" /></el-form-item>
        <el-form-item label="角色">
          <el-select v-model="createDialog.roleCode" style="width: 100%">
            <el-option label="超级管理员" value="admin" />
            <el-option label="家庭管理员" value="family_admin" />
            <el-option label="普通成员" value="member" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="createDialog.roleCode !== 'admin'" label="所属家庭" required>
          <el-select v-model="createDialog.familyId" style="width: 100%">
            <el-option v-for="f in families" :key="f.id" :label="f.name" :value="f.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="createDialog.saving" @click="saveCreate">创建</el-button>
      </template>
    </el-dialog>

    <!-- 编辑用户 -->
    <el-dialog v-model="editDialog.visible" title="编辑用户" width="320px">
      <el-form label-position="top" @submit.prevent="saveEdit">
        <el-form-item label="昵称" required><el-input v-model="editDialog.nickname" /></el-form-item>
        <el-form-item label="邮箱"><el-input v-model="editDialog.email" /></el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="editDialog.status" :active-value="1" :inactive-value="0" active-text="正常" inactive-text="禁用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="editDialog.saving" @click="saveEdit">保存</el-button>
      </template>
    </el-dialog>

    <!-- 设置角色 -->
    <el-dialog v-model="roleDialog.visible" :title="`设置角色：${roleDialog.row?.nickname ?? ''}`" width="340px">
      <el-form label-position="top">
        <el-form-item label="角色">
          <el-select v-model="roleDialog.roleCode" style="width: 100%">
            <el-option label="超级管理员" value="admin" />
            <el-option label="家庭管理员" value="family_admin" />
            <el-option label="普通成员" value="member" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="roleDialog.roleCode !== 'admin'" label="所属家庭" required>
          <el-select v-model="roleDialog.familyId" style="width: 100%">
            <el-option v-for="f in families" :key="f.id" :label="f.name" :value="f.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <el-alert type="info" :closable="false" show-icon title="角色变更后该用户的所有会话会被撤销，需要重新登录" />
      <template #footer>
        <el-button @click="roleDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="roleDialog.saving" @click="saveRole">保存</el-button>
      </template>
    </el-dialog>

    <!-- 删除用户：接收人可选 + 输入昵称确认 -->
    <el-dialog v-model="delDialog.visible" :title="`删除用户：${delDialog.row?.nickname ?? ''}`" width="340px">
      <el-alert
        type="error"
        :closable="false"
        show-icon
        :title="delDialog.row?.familyId && delDialog.receiverOptions.length
          ? '选择接收人则名下物品责任移交（历史保留）；不选则名下物品随用户一并删除。'
          : '该用户名下的物品将随删除一并清除。'"
        style="margin-bottom: 12px"
      />
      <el-form label-position="top" v-if="delDialog.row?.familyId && delDialog.receiverOptions.length">
        <el-form-item label="物品接收人（可选）">
          <el-select v-model="delDialog.receiverId" placeholder="不选则物品一并删除" clearable style="width: 100%">
            <el-option v-for="r in delDialog.receiverOptions" :key="r.id" :label="r.nickname" :value="r.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <el-input
        v-model="delDialog.confirmName"
        :placeholder="`输入昵称「${delDialog.row?.nickname}」确认`"
      />
      <template #footer>
        <el-button @click="delDialog.visible = false">取消</el-button>
        <el-button
          type="danger"
          :loading="delDialog.saving"
          :disabled="delDialog.confirmName.trim() !== delDialog.row?.nickname"
          @click="confirmDelete"
        >
          确认删除
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.filter-bar {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.filter-bar .el-select {
  width: 120px;
  flex-shrink: 0;
}

.user-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.user-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.user-name {
  font-size: 15px;
  font-weight: 600;
}

.user-username {
  font-size: 12px;
  font-weight: 400;
  color: #909399;
  margin-left: 4px;
}

.user-meta {
  margin-top: 6px;
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.user-sub {
  margin-top: 4px;
  font-size: 12px;
  color: #c0c4cc;
}

.user-ops {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px dashed #f0f2f5;
  display: flex;
  justify-content: flex-end;
}
</style>
