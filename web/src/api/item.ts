// 物品与上传 API
import { http } from '@/utils/request'
import type { Item, ItemHistory, Page } from '@/types'

export interface ItemQuery {
  keyword?: string
  categoryId?: number | string
  familyId?: number | string // 仅 admin 生效
  page?: number
  size?: number
}

export function listItems(q: ItemQuery = {}): Promise<Page<Item>> {
  return http.get('/items', { params: q }) as Promise<Page<Item>>
}

export function getItem(id: number | string): Promise<Item> {
  return http.get(`/items/${id}`) as Promise<Item>
}

export function getItemHistory(id: number | string): Promise<ItemHistory[]> {
  return http.get(`/items/${id}/history`) as Promise<ItemHistory[]>
}

export interface ItemPayload {
  name: string
  quantity: number
  categoryId?: number | null
  image?: string | null
  remark?: string
  /** 以下两项仅 admin 代提交时发送：物品进哪个家庭、算哪个成员提交的 */
  familyId?: number
  ownerId?: number
}

export function createItem(payload: ItemPayload): Promise<{ id: number }> {
  return http.post('/items', payload) as Promise<{ id: number }>
}

export function updateItem(id: number | string, payload: ItemPayload): Promise<void> {
  return http.put(`/items/${id}`, payload).then(() => undefined)
}

export function deleteItem(id: number | string): Promise<void> {
  return http.delete(`/items/${id}`).then(() => undefined)
}

/** 图片上传：multipart 字段 file，返回 {url}；前端先传图拿 URL 再随物品提交 */
export function uploadImage(file: File): Promise<string> {
  const form = new FormData()
  form.append('file', file)
  return http
    .post('/uploads', form, { headers: { 'Content-Type': 'multipart/form-data' } })
    .then((d: any) => d.url as string)
}
