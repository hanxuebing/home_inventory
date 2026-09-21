// 物品数据脏标记：编辑 / 创建 / 删除成功时打标，
// 物品列表页（KeepAlive 缓存）从其他页面返回时消费——
// 有标才重新拉数据，纯浏览（详情页看看就回）不发任何请求，滚动位置无损恢复。
let dirty = false

export function markItemsDirty() {
  dirty = true
}

export function consumeItemsDirty(): boolean {
  const d = dirty
  dirty = false
  return d
}
