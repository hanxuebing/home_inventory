<script setup lang="ts">
// "我的"页：用户卡片（角色徽章/家庭）+ 按权限出现的管理入口 + 修改密码 + 退出登录
// 三种角色登录后看到的功能差别从这里开始体现：
//   member        —— 只有修改密码 / 退出
//   family_admin  —— + 成员管理
//   admin         —— + 成员管理 / 家庭管理 / 用户管理 / 审计日志
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowRight, SwitchButton } from '@element-plus/icons-vue'
import { useUserStore } from '@/stores/user'
import { changePassword } from '@/api/auth'
import { roleBadge } from '@/utils/format'

// 显式命名：KeepAlive include 按组件名匹配
// 本页数据全部来自 Pinia store（响应式），缓存后权限变化自动反映，无需 onActivated 刷新
defineOptions({ name: 'ProfileView' })

const router = useRouter()
const store = useUserStore()

const pwDialog = reactive({ visible: false, oldPassword: '', newPassword: '', saving: false })
const loggingOut = ref(false)

/** 管理入口：按权限码动态生成（无权限的入口根本不出现） */
const adminEntries = [
  { to: '/admin/members', label: '成员管理', perm: 'sys:user:page', desc: '管理本家庭成员、移交管理员' },
  { to: '/admin/families', label: '家庭管理', perm: 'sys:family:page', desc: '创建与管理所有家庭' },
  { to: '/admin/users', label: '用户管理', perm: 'sys:family:page', desc: '所有家庭的用户与角色' },
  { to: '/admin/audit', label: '审计日志', perm: 'sys:audit:page', desc: '登录与敏感操作记录' },
].filter((e) => store.hasPerm(e.perm))

async function savePassword() {
  if (pwDialog.newPassword.length < 10) {
    ElMessage.warning('新密码至少 10 位')
    return
  }
  pwDialog.saving = true
  try {
    await changePassword(pwDialog.oldPassword, pwDialog.newPassword)
    ElMessage.success('密码已修改，全部会话已撤销，请重新登录')
    // 改密后端撤销了全部 refresh：本地清状态并回登录页
    store.clear()
    router.replace({ name: 'login' })
  } catch {
    // 错误提示由拦截器弹出
  } finally {
    pwDialog.saving = false
  }
}

async function onLogout() {
  loggingOut.value = true
  await store.logout()
  router.replace({ name: 'login' })
}
</script>

<template>
  <div v-if="store.user">
    <!-- 用户卡片 -->
    <el-card shadow="never" class="me-card">
      <div class="me-row">
        <div class="me-avatar">{{ store.user.nickname.slice(0, 1) }}</div>
        <div class="me-info">
          <div class="me-name">{{ store.user.nickname }}</div>
          <div class="me-meta">@{{ store.user.username }}</div>
          <div class="me-tags">
            <el-tag v-for="r in store.user.roles" :key="r" :type="roleBadge(r).type" size="small">
              {{ roleBadge(r).text }}
            </el-tag>
            <el-tag v-if="store.user.familyName" type="success" size="small" effect="plain">
              {{ store.user.familyName }}
            </el-tag>
          </div>
        </div>
      </div>
    </el-card>

    <!-- 管理入口（按权限动态出现） -->
    <template v-if="adminEntries.length">
      <div class="section-title">管理</div>
      <el-card shadow="never" class="entry-card">
        <div
          v-for="e in adminEntries"
          :key="e.to"
          class="entry-row"
          @click="router.push(e.to)"
        >
          <div>
            <div class="entry-label">{{ e.label }}</div>
            <div class="entry-desc">{{ e.desc }}</div>
          </div>
          <el-icon color="#c0c4cc"><ArrowRight /></el-icon>
        </div>
      </el-card>
    </template>

    <!-- 账号操作 -->
    <div class="section-title">账号</div>
    <el-card shadow="never" class="entry-card">
      <div class="entry-row" @click="pwDialog.visible = true">
        <div class="entry-label">修改密码</div>
        <el-icon color="#c0c4cc"><ArrowRight /></el-icon>
      </div>
      <div class="entry-row danger" @click="onLogout">
        <div class="entry-label">
          <el-icon style="vertical-align: -2px"><SwitchButton /></el-icon>
          退出登录
        </div>
        <el-icon color="#c0c4cc"><ArrowRight /></el-icon>
      </div>
    </el-card>

    <!-- 修改密码对话框：成功后全部会话撤销，强制重新登录 -->
    <el-dialog v-model="pwDialog.visible" title="修改密码" width="320px">
      <el-form label-position="top" @submit.prevent="savePassword">
        <el-form-item label="原密码" required>
          <el-input v-model="pwDialog.oldPassword" type="password" show-password />
        </el-form-item>
        <el-form-item label="新密码（至少 10 位）" required>
          <el-input v-model="pwDialog.newPassword" type="password" show-password />
        </el-form-item>
        <el-alert type="info" :closable="false" show-icon title="修改成功后会撤销所有登录会话，需要重新登录" />
      </el-form>
      <template #footer>
        <el-button @click="pwDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="pwDialog.saving" @click="savePassword">确认修改</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.me-card {
  margin-bottom: 4px;
}

.me-row {
  display: flex;
  gap: 14px;
  align-items: center;
}

.me-avatar {
  width: 56px;
  height: 56px;
  border-radius: 50%;
  background: var(--el-color-primary);
  color: #fff;
  font-size: 22px;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.me-name {
  font-size: 17px;
  font-weight: 700;
}

.me-meta {
  font-size: 12px;
  color: #909399;
  margin: 2px 0 6px;
}

.me-tags {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.entry-card :deep(.el-card__body) {
  padding: 0;
}

.entry-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 14px;
  min-height: 52px;
  border-bottom: 1px solid #f0f2f5;
  cursor: pointer;
}

.entry-row:last-child {
  border-bottom: none;
}

.entry-row.danger .entry-label {
  color: var(--el-color-danger);
}

.entry-label {
  font-size: 15px;
  color: #303133;
}

.entry-desc {
  font-size: 12px;
  color: #c0c4cc;
  margin-top: 2px;
}
</style>
