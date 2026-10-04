<script setup lang="ts">
import Button from 'primevue/button'
import DatePicker from 'primevue/datepicker'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import AppBreadcrumb, { type Crumb } from '@/components/AppBreadcrumb.vue'
import { fromDateString, toDateString } from '@/lib/dates'
import { isApiError } from '@/lib/errors'
import { useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { useAssetType, useAssetTypes } from '@/features/asset-types/api'
import { useStatuses } from '@/features/statuses/api'
import { useAsset, useCreateAsset, useReplaceAsset } from '../api'
import AttributeInput from '../components/AttributeInput.vue'
import { useListContext } from '../listContext'
import { typeListLocation } from '../listQuery'
import { type FormValues, fromApiValues, toApiValues } from '../values'

// Không có id là tạo mới; có id là sửa (PUT thay toàn bộ)
// typeId (từ /types/:typeId/assets/new): chọn sẵn loại cho tài sản mới
const props = defineProps<{ id?: string; typeId?: string }>()

const router = useRouter()
const listContext = useListContext()
const errors = useFormErrors()
const isEdit = computed(() => !!props.id)
const { data: asset, refetch } = useAsset(() => props.id)

const tag = ref('')
const name = ref('')
const description = ref('')
const typeId = ref<string | null>(props.typeId ?? null)
// null: tạo mới thì backend chọn status mặc định, sửa thì giữ nguyên
const statusId = ref<string | null>(null)
const purchaseDate = ref<Date | null>(null)
const values = ref<FormValues>({})

// Loại đã lưu trữ chỉ hiện khi tài sản đang dùng nó
const { data: types } = useAssetTypes(true)
const typeOptions = computed(() =>
  (types.value ?? []).filter((t) => !t.archived_at || t.id === asset.value?.asset_type.id),
)
// Status kind retired chỉ đặt bằng thao tác Retire
const { data: statuses } = useStatuses(true)
const statusOptions = computed(() =>
  (statuses.value ?? []).filter(
    (s) => s.kind !== 'retired' && (!s.archived_at || s.id === asset.value?.status.id),
  ),
)

const { data: selectedType } = useAssetType(() => typeId.value ?? undefined)
const attributes = computed(() =>
  selectedType.value && selectedType.value.id === typeId.value
    ? selectedType.value.attributes.filter((a) => !a.removed).sort((a, b) => a.position - b.position)
    : [],
)
const typeChanged = computed(() => isEdit.value && !!asset.value && typeId.value !== asset.value.asset_type.id)

// valuesFor: loại mà values đang dựng theo; null là cần dựng lại
let valuesFor: string | null = null

// Đổ dữ liệu tài sản vào form khi tải xong, và sau khi tải lại (409)
watch(
  () => asset.value?.version,
  () => {
    const a = asset.value
    if (!a) return
    tag.value = a.tag
    name.value = a.name
    description.value = a.description
    typeId.value = a.asset_type.id
    statusId.value = a.status.id
    purchaseDate.value = fromDateString(a.purchase_date)
    valuesFor = null
  },
  { immediate: true },
)

// Giá trị thuộc tính dựng lại mỗi khi loại đổi; về lại loại cũ thì lấy lại giá trị đã lưu
watch(
  [attributes, typeId],
  ([attrs, t]) => {
    if (!t || !selectedType.value || selectedType.value.id !== t || valuesFor === t) return
    valuesFor = t
    const saved = asset.value && asset.value.asset_type.id === t ? asset.value.attributes : []
    values.value = fromApiValues(attrs, saved)
  },
  { immediate: true },
)

const create = useCreateAsset()
const replace = useReplaceAsset()
const busy = computed(() => create.isPending.value || replace.isPending.value)

async function submit() {
  if (!typeId.value) return
  errors.clear()
  const body = {
    name: name.value,
    description: description.value,
    asset_type_id: typeId.value,
    status_id: statusId.value ?? undefined,
    purchase_date: toDateString(purchaseDate.value),
    // form không hiện vị trí và người giữ, nhưng PUT thay toàn bộ: gửi lại giá trị cũ
    location_id: asset.value?.location_id,
    holder_member_id: asset.value?.holder_member_id,
    attributes: toApiValues(attributes.value, values.value),
  }
  try {
    const saved =
      isEdit.value && asset.value
        ? await replace.mutateAsync({ id: asset.value.id, version: asset.value.version, ...body })
        : await create.mutateAsync({ tag: tag.value, ...body })
    notify.success('Asset saved.')
    // tài sản mới: mở trang của nó thay cho form; sửa: quay về nơi đã mở form
    if (isEdit.value) leave(`/assets/${saved.id}`)
    else await router.replace(`/assets/${saved.id}`)
  } catch (err) {
    if (isApiError(err, '/errors/asset-changed')) {
      notify.info('Someone else changed this asset. Reloaded the latest version; please re-apply your edits.')
      await refetch()
    } else {
      errors.set(err)
    }
  }
}

// leave: quay về trang trước (danh sách hay trang tài sản) để form không ở lại trong
// lịch sử; mở form trực tiếp (không có trang trước) thì sang trang dự phòng
function leave(fallback: string) {
  if (window.history.state?.back) router.back()
  else router.replace(fallback)
}

const crumbs = computed<Crumb[]>(() => {
  const t = (types.value ?? []).find((x) => x.id === (asset.value?.asset_type.id ?? typeId.value))
  const items: Crumb[] = [{ label: 'Assets', to: '/assets' }]
  if (t) items.push({ label: t.name, to: typeListLocation(t.id, listContext.views) })
  if (isEdit.value && asset.value) items.push({ label: asset.value.tag, to: `/assets/${asset.value.id}` }, { label: 'Edit' })
  else if (!isEdit.value) items.push({ label: 'New asset' })
  return items
})
</script>

<template>
  <section>
    <AppBreadcrumb :items="crumbs" />
    <h1>{{ isEdit ? `Edit ${asset?.tag ?? ''}` : 'New asset' }}</h1>
    <form v-if="!isEdit || asset" class="form" @submit.prevent="submit">
      <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>

      <div class="field">
        <label for="tag">Tag</label>
        <InputText id="tag" v-model="tag" :disabled="isEdit" required placeholder="LAP-0001" />
        <small v-if="!isEdit">A-Z, 0-9, '.', '_' or '-'. Can't be changed later.</small>
        <small v-if="errors.fields.value.tag" class="field-error">{{ errors.fields.value.tag }}</small>
      </div>
      <div class="field">
        <label for="name">Name</label>
        <InputText id="name" v-model="name" required />
        <small v-if="errors.fields.value.name" class="field-error">{{ errors.fields.value.name }}</small>
      </div>
      <div class="field">
        <label for="description">Description</label>
        <Textarea id="description" v-model="description" rows="3" auto-resize />
      </div>
      <div class="field">
        <label for="type">Type</label>
        <Select
          v-model="typeId"
          input-id="type"
          :options="typeOptions"
          option-label="name"
          option-value="id"
          placeholder="Choose a type"
          filter
        />
        <Message v-if="typeChanged" severity="warn">
          Changing the type drops all attribute values of the old type when you save.
        </Message>
        <small v-if="errors.fields.value.asset_type_id" class="field-error">{{ errors.fields.value.asset_type_id }}</small>
      </div>
      <div class="field">
        <label for="status">Status</label>
        <Select
          v-model="statusId"
          input-id="status"
          :options="statusOptions"
          option-label="name"
          option-value="id"
          :placeholder="isEdit ? 'Keep current' : 'Default (available)'"
          :show-clear="!isEdit"
        />
        <small v-if="errors.fields.value.status_id" class="field-error">{{ errors.fields.value.status_id }}</small>
      </div>
      <div class="field">
        <label for="purchase-date">Purchase date</label>
        <DatePicker v-model="purchaseDate" input-id="purchase-date" date-format="yy-mm-dd" show-icon show-button-bar />
        <small v-if="errors.fields.value.purchase_date" class="field-error">{{ errors.fields.value.purchase_date }}</small>
      </div>

      <fieldset v-if="attributes.length">
        <legend>{{ selectedType?.name }} attributes</legend>
        <div class="form">
          <AttributeInput
            v-for="a in attributes"
            :key="a.id"
            v-model="values[a.key]"
            :attribute="a"
            :error="errors.fields.value[`attributes.${a.key}`]"
          />
        </div>
      </fieldset>

      <div class="actions">
        <Button type="submit" label="Save" :loading="busy" :disabled="!typeId" />
        <Button label="Cancel" severity="secondary" text @click="leave(isEdit ? `/assets/${id}` : props.typeId ? `/types/${props.typeId}/assets` : '/assets')" />
      </div>
    </form>
  </section>
</template>
