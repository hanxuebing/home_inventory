<script setup lang="ts">
// 成员管理：家庭管理员视角（admin 亦可从用户管理跳入指定家庭）。
// 功能：成员列表（角色徽章）、添加成员、编辑、删除（接收人可选，输入昵称确认）、移交家庭管理员
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { familyMembers, transferFamilyAdmin, createUser, updateUser, deleteUser } from '@/api/admin'
import { useUserStore } from '@/stores/user'
import { roleBadge } from '@/utils/format'
import type { FamilyMember } from '@/types'
const route = useRoute()
const router = useRouter()
const store = useUserStore()

/** 目标家庭：路由带 familyId（admin 跳入）否则自己的家庭 */
const familyId = computed(() => {
  if (route.query.familyId) return Number(route.query.familyId)
  return store.user?.familyId ?? null
})

/** 是否 admin（admin 可以在这里为家庭添加成员） */
const isAdmin = store.hasRole('admin')

const members = ref<FamilyMember[]>([])
const loading = ref(true)

// ---- 添加成员 ----
// asAdmin：仅 admin 可见 —— 创建账号的同时设为家庭管理员（后端单管理员模型，已有管理员会拒绝）
const createDialog = reactive({
  visible: false,
  username: '',
  password: '',
  nickname: '',
  asAdmin: false,
  saving: false,
})

// ---- 编辑成员 ----
const editDialog = reactive({ visible: false, id: 0, nickname: '', email: '', status: 1, saving: false })

// ---- 删除成员（接收人可选 + 输入昵称确认） ----
const delDialog = reactive({
  visible: false,
  member: null as FamilyMember | null,
  receiverId: null as number | null,
  confirmName: '',
  saving: false,
})

// ---- 移交家庭管理员 ----
const transferDialog = reactive({ visible: false, target: null as FamilyMember | null, saving: false })

/** 删除对话框里的接收人候选：同家庭其他人 */
const receiverOptions = computed(() =>
  delDialog.member ? members.value.filter((m) => m.id !== delDialog.member!.id) : [],
)

async function load() {
  if (!familyId.value) return
  loading.value = true
  try {
    members.value = await familyMembers(familyId.value)
  } finally {
    loading.value = false
  }
}

async function saveCreate() {
  if (createDialog.password.length < 10) {
    ElMessage.warning('初始密码至少 10 位')
    return
  }
  createDialog.saving = true
  try {
    // family_admin 调用时后端强制本家庭 + member 角色；admin 可建任意家庭成员并指定角色
    await createUser({
      username: createDialog.username.trim(),
      password: createDialog.password,
      nickname: createDialog.nickname.trim(),
      familyId: isAdmin ? familyId.value : undefined,
      roleCode: isAdmin && createDialog.asAdmin ? 'family_admin' : undefined,
    })
    ElMessage.success(createDialog.asAdmin && isAdmin ? '家庭管理员已创建' : '成员已添加')
    createDialog.visible = false
    Object.assign(createDialog, { username: '', password: '', nickname: '', asAdmin: false })
    load()
  } catch {
    // 提示由拦截器弹出
  } finally {
    createDialog.saving = false
  }
}

/** admin 代成员提交物品：跳录入页并带上家庭与提交人身份 */
function submitFor(m: FamilyMember) {
  router.push({ path: '/items/new', query: { familyId: familyId.value, ownerId: m.id } })
}

function openEdit(m: FamilyMember) {
  Object.assign(editDialog, { visible: true, id: m.id, nickname: m.nickname, email: '', status: 1, saving: false })
}

async function saveEdit() {
  editDialog.saving = true
  try {
    await updateUser(editDialog.id, { nickname: editDialog.nickname, status: editDialog.status })
    ElMessage.success('已保存')
    editDialog.visible = false
    load()
  } catch {
  } finally {
    editDialog.saving = false
  }
}

function openDelete(m: FamilyMember) {
  Object.assign(delDialog, { visible: true, member: m, receiverId: null, confirmName: '' })
}

async function confirmDelete() {
  const m = delDialog.member!
  delDialog.saving = true
  try {
    const { transferred, purged } = await deleteUser(m.id, {
      receiverId: delDialog.receiverId,
      confirmName: delDialog.confirmName.trim(),
    })
    ElMessage.success(
      transferred > 0
        ? `已删除，${transferred} 件物品的责任已移交`
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

function openTransfer(m: FamilyMember) {
  Object.assign(transferDialog, { visible: true, target: m })
}

async function confirmTransfer() {
  const t = transferDialog.target!
  transferDialog.saving = true
  try {
    await transferFamilyAdmin(familyId.value!, t.id)
    ElMessage.success(`已移交家庭管理员给 ${t.nickname}，双方需重新登录`)
    transferDialog.visible = false
    // 若移交的是自己：本会话已被后端撤销，回登录页
    if (t.id === store.user?.id) {
      store.clear()
      router.replace({ name: 'login' })
    } else {
      load()
    }
  } catch {
  } finally {
    transferDialog.saving = false
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="mb-head">
      <span class="mb-title">本家庭成员</span>
      <el-button type="primary" size="small" @click="createDialog.visible = true">添加成员</el-button>
    </div>

    <div v-if="loading" class="page-loading"><el-spinner /></div>

    <div v-else class="mb-list">
      <el-card v-for="m in members" :key="m.id" shadow="never">
        <div class="mb-row">
          <div class="mb-info">
            <div class="mb-name">
              {{ m.nickname }}
              <el-tag
                v-for="r in m.roles"
                :key="r"
                :type="roleBadge(r).type"
                size="small"
                style="margin-left: 6px"
              >
                {{ roleBadge(r).text }}
              </el-tag>
            </div>
            <div class="mb-meta">@{{ m.username }}</div>
          </div>
          <div class="mb-ops">
            <el-button text type="primary" size="small" @click="openEdit(m)">编辑</el-button>
            <el-button
              v-if="isAdmin"
              text
              type="success"
              size="small"
              @click="submitFor(m)"
            >
              代提交
            </el-button>
            <el-button
              v-if="!m.roles.includes('admin') && !m.roles.includes('family_admin')"
              text
              type="warning"
              size="small"
              @click="openTransfer(m)"
            >
              设为管理员
            </el-button>
            <el-button
              v-if="!m.roles.includes('admin') && m.id !== store.user?.id"
              text
              type="danger"
              size="small"
              @click="openDelete(m)"
            >
              删除
            </el-button>
          </div>
        </div>
      </el-card>
    </div>

    <!-- 添加成员 -->
    <el-dialog v-model="createDialog.visible" title="添加家庭成员" width="320px">
      <el-form label-position="top" @submit.prevent="saveCreate">
        <el-form-item label="账号" required>
          <el-input v-model="createDialog.username" placeholder="登录账号（>=3 位）" />
        </el-form-item>
        <el-form-item label="初始密码" required>
          <el-input v-model="createDialog.password" type="password" show-password placeholder=">=10 位，成员可自行修改" />
        </el-form-item>
        <el-form-item label="昵称" required>
          <el-input v-model="createDialog.nickname" placeholder="如：张小妹" />
        </el-form-item>
        <el-form-item v-if="isAdmin" label="角色">
          <el-switch
            v-model="createDialog.asAdmin"
            active-text="家庭管理员"
            inactive-text="普通成员"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="createDialog.saving" @click="saveCreate">添加</el-button>
      </template>
    </el-dialog>

    <!-- 编辑成员 -->
    <el-dialog v-model="editDialog.visible" title="编辑成员" width="320px">
      <el-form label-position="top" @submit.prevent="saveEdit">
        <el-form-item label="昵称" required>
          <el-input v-model="editDialog.nickname" />
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="editDialog.status" :active-value="1" :inactive-value="0" active-text="正常" inactive-text="禁用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="editDialog.saving" @click="saveEdit">保存</el-button>
      </template>
    </el-dialog>

    <!-- 删除成员：接收人可选 + 输入昵称确认 -->
    <el-dialog v-model="delDialog.visible" :title="`删除成员：${delDialog.member?.nickname ?? ''}`" width="340px">
      <el-alert
        type="error"
        :closable="false"
        show-icon
        :title="receiverOptions.length
          ? '选择接收人则名下物品责任移交（历史保留）；不选则名下物品随成员一并删除。'
          : '该成员名下的物品将随删除一并清除。'"
        style="margin-bottom: 12px"
      />
      <el-form label-position="top" v-if="receiverOptions.length">
        <el-form-item label="物品接收人（可选）">
          <el-select v-model="delDialog.receiverId" placeholder="不选则物品一并删除" clearable style="width: 100%">
            <el-option v-for="r in receiverOptions" :key="r.id" :label="r.nickname" :value="r.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <el-input
        v-model="delDialog.confirmName"
        :placeholder="`输入昵称「${delDialog.member?.nickname}」确认`"
      />
      <template #footer>
        <el-button @click="delDialog.visible = false">取消</el-button>
        <el-button
          type="danger"
          :loading="delDialog.saving"
          :disabled="delDialog.confirmName.trim() !== delDialog.member?.nickname"
          @click="confirmDelete"
        >
          确认删除
        </el-button>
      </template>
    </el-dialog>

    <!-- 移交家庭管理员 -->
    <el-dialog v-model="transferDialog.visible" title="移交家庭管理员" width="340px">
      <p style="font-size: 14px; color: #606266; line-height: 1.8">
        将家庭管理员移交给 <b>{{ transferDialog.target?.nickname }}</b>？移交后你将变为普通成员，
        双方都需要重新登录。
      </p>
      <template #footer>
        <el-button @click="transferDialog.visible = false">取消</el-button>
        <el-button type="warning" :loading="transferDialog.saving" @click="confirmTransfer">确认移交</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.mb-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.mb-title {
  font-size: 14px;
  font-weight: 600;
}

.mb-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.mb-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.mb-name {
  font-size: 15px;
  font-weight: 600;
}

.mb-meta {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
}

.mb-ops {
  flex-shrink: 0;
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
}
</style>
