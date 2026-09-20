// v-permission 按钮级权限指令（设计稿 §5）：
// 无权限码则直接移除 DOM —— 路由管"进不进得来"，指令管"看不看得见"，接口管"调不调得动"
import type { Directive } from 'vue'
import { useUserStore } from '@/stores/user'

export const permission: Directive<HTMLElement, string | string[]> = {
  mounted(el, binding) {
    const store = useUserStore()
    const need = Array.isArray(binding.value) ? binding.value : [binding.value]
    const ok = need.some((code) => store.hasPerm(code))
    if (!ok) {
      el.parentNode?.removeChild(el)
    }
  },
}

export default permission
