import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import 'element-plus/dist/index.css'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import App from './App.vue'
import router from './router'
import permission from './directives/permission'
import { bindAuth } from './utils/request'
import { useUserStore } from './stores/user'
import './styles.css'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(ElementPlus, { locale: zhCn })

// 全量注册图标（按需引入对学习项目收益不大）
for (const [name, comp] of Object.entries(ElementPlusIconsVue)) {
  app.component(name, comp)
}

// 按钮级权限指令：v-permission="'biz:item:delete'"
app.directive('permission', permission)

// request 拦截器与 user store 绑定：
// - token 注入（内存）；- 刷新成功写回 store；- 会话彻底失效时清状态并跳登录页
const userStore = useUserStore()
bindAuth(
  () => userStore.token,
  () => {
    userStore.clear()
    const cur = router.currentRoute.value
    if (cur.name !== 'login') {
      router.push({ name: 'login', query: { redirect: cur.fullPath } })
    }
  },
  (t) => {
    userStore.token = t
  },
)

app.mount('#app')
