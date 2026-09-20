// 分类 API
import { http } from '@/utils/request'
import type { Category } from '@/types'

export function listCategories(familyId?: number | string): Promise<Category[]> {
  return http.get('/categories', { params: familyId ? { familyId } : {} }) as Promise<Category[]>
}

export function createCategory(name: string, sort = 0, familyId?: number): Promise<{ id: number }> {
  return http.post('/categories', { name, sort, familyId }) as Promise<{ id: number }>
}

export function updateCategory(id: number | string, name: string, sort = 0): Promise<void> {
  return http.put(`/categories/${id}`, { name, sort }).then(() => undefined)
}

export function deleteCategory(id: number | string): Promise<void> {
  return http.delete(`/categories/${id}`).then(() => undefined)
}
