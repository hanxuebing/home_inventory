<script setup lang="ts">
// 登录页：提交后 token 与 user 入内存 store，refresh 已由后端写入 httpOnly Cookie
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { User, Lock } from '@element-plus/icons-vue'
import { useUserStore } from '@/stores/user'

const router = useRouter()
const route = useRoute()
const store = useUserStore()

const form = reactive({ username: '', password: '' })
const loading = ref(false)

async function submit() {
  if (!form.username || !form.password) {
    ElMessage.warning('请输入账号和密码')
    return
  }
  loading.value = true
  try {
    await store.login(form.username.trim(), form.password)
    const redirect = (route.query.redirect as string) || '/items'
    router.replace(redirect)
  } catch {
    // 错误提示由 request 拦截器统一弹出（统一文案"账号或密码错误"由后端保证）
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-card">
      <div class="login-logo">🏠</div>
      <h1 class="login-title">家庭物品整理收纳</h1>
      <p class="login-sub">登录后与家人共享家里的每一件物品</p>

      <el-form @submit.prevent="submit">
        <el-input
          v-model="form.username"
          size="large"
          placeholder="账号"
          :prefix-icon="User"
          autocomplete="username"
          class="login-input"
        />
        <el-input
          v-model="form.password"
          size="large"
          type="password"
          placeholder="密码"
          :prefix-icon="Lock"
          show-password
          autocomplete="current-password"
          class="login-input"
          @keyup.enter="submit"
        />
        <el-button type="primary" size="large" class="login-btn" :loading="loading" @click="submit">
          登 录
        </el-button>
      </el-form>

      <div class="login-hint">
        <p>演示账号（密码即口令本身）：</p>
        <p>admin / Admin@123456 —— 超级管理员</p>
        <p>zhangsan / Family@123456 —— 家庭管理员</p>
        <p>zhangmei / Member@123456 —— 普通成员</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  min-height: 100dvh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px 16px;
  background: linear-gradient(180deg, #e8f1ff 0%, #f5f7fa 40%);
}

.login-card {
  width: 100%;
  max-width: 400px;
  background: #fff;
  border-radius: 16px;
  padding: 36px 28px 28px;
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.06);
  text-align: center;
}

.login-logo {
  font-size: 44px;
  line-height: 1;
}

.login-title {
  margin: 12px 0 4px;
  font-size: 20px;
  color: #303133;
}

.login-sub {
  margin: 0 0 24px;
  font-size: 13px;
  color: #909399;
}

.login-input {
  margin-bottom: 14px;
}

.login-btn {
  width: 100%;
  margin-top: 4px;
}

.login-hint {
  margin-top: 22px;
  padding-top: 14px;
  border-top: 1px dashed #e4e7ed;
  text-align: left;
  font-size: 12px;
  color: #909399;
  line-height: 1.9;
}

.login-hint p {
  margin: 0;
}
</style>
