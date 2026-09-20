<script setup lang="ts">
// 物品编辑：新增 / 编辑二合一（有 :id 路由参数则为编辑）。
// 图片：el-upload 自定义 http-request → POST /uploads 拿 URL，再随物品 JSON 一起提交
import { onMounted, reactive, ref, watch, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { UploadRequestOptions } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { getItem, createItem, updateItem, uploadImage } from '@/api/item'
import { listCategories, createCategory } from '@/api/category'
import { listFamilies, familyMembers } from '@/api/admin'
import { useUserStore } from '@/stores/user'
import type { Category, Family, FamilyMember } from '@/types'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const isEdit = ref(false)
const itemId = ref<number | null>(null)
const categories = ref<Category[]>([])
const submitting = ref(false)
const uploading = ref(false)

/** admin 代提交模式：admin 自己没有家庭，录入时必须指定家庭 + 提交成员 */
const adminMode = computed(() => !isEdit.value && userStore.hasRole('admin'))

// ---- admin 代提交：家庭与成员选择 ----
const families = ref<Family[]>([])
const members = ref<FamilyMember[]>([])
const membersLoading = ref(false)

const form = reactive({
  name: '',
  quantity: 1,
  categoryId: null as number | null,
  image: null as string | null,
  remark: '',
  familyId: null as number | null, // 仅 admin 用
  ownerId: null as number | null, // 仅 admin 用：以该成员身份提交
})

onMounted(async () => {
  if (route.params.id) {
    isEdit.value = true
    const it = await getItem(route.params.id as string)
    itemId.value = it.id
    form.name = it.name
    form.quantity = it.quantity
    form.categoryId = it.categoryId
    form.image = it.image
    form.remark = it.remark
    categories.value = await loadCategoriesFor(isEdit.value ? it.familyId : undefined)
    return
  }

  // 新增：admin 从成员列表"代提交"跳入时带 query，预填家庭与成员
  if (userStore.hasRole('admin')) {
    families.value = await listFamilies()
    if (route.query.familyId) form.familyId = Number(route.query.familyId)
    if (route.query.ownerId) form.ownerId = Number(route.query.ownerId)
    if (form.familyId) await loadMembers(form.familyId)
  }
  categories.value = await loadCategoriesFor(form.familyId ?? undefined)
})

/** 分类按目标家庭加载（admin 全量接口不传 familyId 会返回所有家庭的分类） */
function loadCategoriesFor(familyId?: number): Promise<Category[]> {
  return listCategories(familyId)
}

/** admin 切换目标家庭：联动成员列表 + 分类列表，清空已选 */
watch(
  () => form.familyId,
  async (fid, old) => {
    if (!adminMode.value || fid === old) return
    form.ownerId = null
    form.categoryId = null
    if (fid) {
      await loadMembers(fid)
      categories.value = await loadCategoriesFor(fid)
    } else {
      members.value = []
      categories.value = []
    }
  },
)

async function loadMembers(familyId: number) {
  membersLoading.value = true
  try {
    members.value = await familyMembers(familyId)
  } finally {
    membersLoading.value = false
  }
}

/** 自定义上传：调 POST /uploads，移动端 accept="image/*" 可直接调相机 */
async function customUpload(opt: UploadRequestOptions) {
  uploading.value = true
  try {
    form.image = await uploadImage(opt.file)
  } catch {
    // 错误提示由拦截器弹出
  } finally {
    uploading.value = false
  }
}

function removeImage() {
  form.image = null
}

/** 快速新建分类：弹窗输入名称 → POST /categories → 刷新列表并自动选中。
 *  admin 代提交时分类必须挂到目标家庭（未选家庭先提示去选）；
 *  member 没有 biz:category:create 权限码，按钮不显示（后端也会拦）。 */
async function quickAddCategory() {
  if (adminMode.value && !form.familyId) {
    ElMessage.warning('请先选择家庭')
    return
  }
  let name: string
  try {
    const { value } = await ElMessageBox.prompt('分类名（如：厨房用品）', '新建分类', {
      confirmButtonText: '创建',
      cancelButtonText: '取消',
      inputPattern: /\S+/,
      inputErrorMessage: '分类名不能为空',
      inputValidator: (v: string) => (v?.trim().length ?? 0) <= 64 || '不能超过 64 字',
    })
    name = value.trim()
  } catch {
    return // 用户取消
  }
  try {
    const { id } = await createCategory(
      name,
      0,
      adminMode.value ? form.familyId! : undefined,
    )
    categories.value = await loadCategoriesFor(form.familyId ?? undefined)
    form.categoryId = id // 自动选中新建的分类
    ElMessage.success(`分类「${name}」已创建`)
  } catch {
    // 错误提示由拦截器弹出（如分类名重复）
  }
}

async function submit() {
  if (!form.name.trim()) {
    ElMessage.warning('请填写物品名称')
    return
  }
  if (adminMode.value && (!form.familyId || !form.ownerId)) {
    ElMessage.warning('超管提交物品必须指定家庭与家庭成员')
    return
  }
  submitting.value = true
  try {
    const payload = {
      name: form.name.trim(),
      quantity: form.quantity,
      categoryId: form.categoryId,
      image: form.image,
      remark: form.remark,
      // admin 代提交：指定归属家庭与提交人（后端校验成员属于该家庭）
      familyId: adminMode.value ? form.familyId! : undefined,
      ownerId: adminMode.value ? form.ownerId! : undefined,
    }
    if (isEdit.value && itemId.value) {
      await updateItem(itemId.value, payload)
      ElMessage.success('已保存')
    } else {
      await createItem(payload)
      ElMessage.success('已录入')
    }
    router.back()
  } catch {
    // 错误提示由拦截器弹出
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div>
    <el-card shadow="never">
      <el-alert
        v-if="adminMode"
        type="info"
        :closable="false"
        show-icon
        title="超管代提交：物品将以所选成员的身份录入该家庭（提交人 / 责任人均为该成员）"
        style="margin-bottom: 12px"
      />
      <el-form label-position="top">
        <template v-if="adminMode">
          <el-form-item label="家庭" required>
            <el-select v-model="form.familyId" placeholder="选择家庭" style="width: 100%">
              <el-option v-for="f in families" :key="f.id" :label="f.name" :value="f.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="提交成员" required>
            <el-select
              v-model="form.ownerId"
              placeholder="先选择家庭"
              :disabled="!form.familyId"
              :loading="membersLoading"
              style="width: 100%"
            >
              <el-option
                v-for="m in members"
                :key="m.id"
                :label="`${m.nickname}（@${m.username}）`"
                :value="m.id"
              />
            </el-select>
          </el-form-item>
        </template>

        <el-form-item label="物品名称" required>
          <el-input v-model="form.name" placeholder="如：不粘炒锅" maxlength="128" />
        </el-form-item>

        <el-form-item label="数量">
          <el-input-number v-model="form.quantity" :min="0" :max="99999" style="width: 100%" />
        </el-form-item>

        <el-form-item label="分类（可不选）">
          <div class="category-row">
            <el-select v-model="form.categoryId" placeholder="未分类" clearable class="category-select">
              <el-option v-for="c in categories" :key="c.id" :label="c.name" :value="c.id" />
            </el-select>
            <el-button
              v-if="userStore.hasPerm('biz:category:create')"
              class="category-add"
              @click="quickAddCategory"
            >
              <el-icon><Plus /></el-icon>
              新建
            </el-button>
          </div>
        </el-form-item>

        <el-form-item label="图片（可不传，支持拍照）">
          <div class="upload-row">
            <div v-if="form.image" class="upload-preview">
              <img :src="form.image" alt="" />
              <button class="preview-del" type="button" @click="removeImage">×</button>
            </div>
            <el-upload
              v-else
              :show-file-list="false"
              :http-request="customUpload"
              accept="image/jpeg,image/png,image/webp"
              class="upload-trigger"
            >
              <div v-loading="uploading" class="upload-box">
                <el-icon :size="22"><Plus /></el-icon>
                <span>{{ uploading ? '上传中' : '添加图片' }}</span>
              </div>
            </el-upload>
          </div>
        </el-form-item>

        <el-form-item label="备注">
          <el-input
            v-model="form.remark"
            type="textarea"
            :rows="3"
            maxlength="1024"
            show-word-limit
            placeholder="放哪了、什么时候买的、注意事项…"
          />
        </el-form-item>
      </el-form>

      <el-button type="primary" size="large" style="width: 100%" :loading="submitting" @click="submit">
        {{ isEdit ? '保存修改' : '录入' }}
      </el-button>
    </el-card>
  </div>
</template>

<style scoped>
.category-row {
  display: flex;
  gap: 8px;
  width: 100%;
}
.category-select {
  flex: 1;
}
.category-add {
  flex-shrink: 0;
}

.upload-row {
  display: flex;
  gap: 12px;
}

.upload-box {
  width: 96px;
  height: 96px;
  border: 1px dashed #c0c4cc;
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  color: #909399;
  font-size: 12px;
  cursor: pointer;
}

.upload-preview {
  position: relative;
  width: 96px;
  height: 96px;
  border-radius: 8px;
  overflow: hidden;
}

.upload-preview img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.preview-del {
  position: absolute;
  top: 0;
  right: 0;
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 0 0 0 8px;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  font-size: 16px;
  cursor: pointer;
}
</style>
