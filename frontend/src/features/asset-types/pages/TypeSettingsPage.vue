<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Column from 'primevue/column'
import DataTable, { type DataTableRowReorderEvent } from 'primevue/datatable'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Tag from 'primevue/tag'
import Textarea from 'primevue/textarea'
import { useConfirm } from 'primevue/useconfirm'
import { computed, ref, watch } from 'vue'
import { useTabTitle } from '@/app/tabs/tabPage'
import AppBreadcrumb, { type Crumb } from '@/components/AppBreadcrumb.vue'
import type { Attribute } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { isApiError } from '@/lib/errors'
import { useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import {
  useArchiveAssetType,
  useAssetType,
  useRemoveAttribute,
  useReorderAttributes,
  useRestoreAssetType,
  useUpdateAssetType,
} from '../api'
import AttributeDialog from '../components/AttributeDialog.vue'
import OptionsDialog from '../components/OptionsDialog.vue'
import { useListContext } from '@/features/assets/listContext'
import { typeListLocation } from '@/features/assets/listQuery'

// Cài đặt của một loại: tên, mô tả, thuộc tính và option, lưu trữ
const props = defineProps<{ typeId: string }>()

const session = useSession()
const confirm = useConfirm()
const canManage = computed(() => session.can(Perm.TypeManage))

const { data: type, refetch } = useAssetType(() => props.typeId)

const showRemoved = ref(false)
// thứ tự đang hiện: kéo thả đổi ngay, không đợi tải lại
const attributes = ref<Attribute[]>([])
watch(
  [() => type.value?.attributes, showRemoved],
  ([list, removed]) => {
    attributes.value = [...(list ?? [])].filter((a) => removed || !a.removed).sort((a, b) => a.position - b.position)
  },
  { immediate: true },
)
// Kéo thả thứ tự thuộc tính (thứ tự cột trong danh sách và trong form)
const reorder = useReorderAttributes()
function onReorder(e: DataTableRowReorderEvent) {
  attributes.value = e.value as Attribute[]
  reorder.mutate({ typeId: props.typeId, ids: attributes.value.filter((a) => !a.removed).map((a) => a.id) })
}
const nextPosition = computed(() => Math.max(0, ...(type.value?.attributes ?? []).map((a) => a.position)) + 1)

// Sửa tên, mô tả (version: 409 khi người khác vừa sửa)
const name = ref('')
const description = ref('')
watch(
  type,
  (t) => {
    if (!t) return
    name.value = t.name
    description.value = t.description
  },
  { immediate: true },
)
const errors = useFormErrors()
const update = useUpdateAssetType()
async function saveDetails() {
  if (!type.value) return
  errors.clear()
  try {
    await update.mutateAsync({ id: props.typeId, name: name.value, description: description.value, version: type.value.version })
    notify.success('Asset type saved.')
  } catch (err) {
    if (isApiError(err) && err.status === 409) {
      notify.info('Someone else changed this type. Reloaded the latest version.')
      await refetch()
    } else {
      errors.set(err)
    }
  }
}

const archive = useArchiveAssetType()
const restore = useRestoreAssetType()
function toggleArchived() {
  const t = type.value
  if (!t) return
  if (t.archived_at) {
    restore
      .mutateAsync(t.id)
      .then(() => notify.success('Asset type restored.'))
      .catch(() => {})
    return
  }
  confirm.require({
    message: `Archive ${t.name}? It can't be chosen for new assets; existing assets keep it.`,
    header: 'Confirm',
    acceptLabel: 'Archive',
    rejectLabel: 'Cancel',
    accept: () =>
      archive
        .mutateAsync(t.id)
        .then(() => notify.success('Asset type archived.'))
        .catch(() => {}),
  })
}

// Dialog thêm/sửa thuộc tính và dialog option
const attrOpen = ref(false)
const attrEditing = ref<Attribute | null>(null)
function openAttribute(a: Attribute | null) {
  attrEditing.value = a
  attrOpen.value = true
}
const optionsOpen = ref(false)
const optionsAttrId = ref<string | null>(null)
const optionsAttr = computed(() => type.value?.attributes.find((a) => a.id === optionsAttrId.value) ?? null)
function openOptions(a: Attribute) {
  optionsAttrId.value = a.id
  optionsOpen.value = true
}

const removeAttr = useRemoveAttribute()
function askRemove(a: Attribute) {
  confirm.require({
    message: `Remove the attribute ${a.label}? Existing values are kept but hidden.`,
    header: 'Confirm',
    acceptLabel: 'Remove',
    rejectLabel: 'Cancel',
    accept: () =>
      removeAttr
        .mutateAsync({ typeId: props.typeId, attrId: a.id })
        .then(() => notify.success('Attribute removed.'))
        .catch(() => {}),
  })
}

const listContext = useListContext()
useTabTitle(() => type.value && `${type.value.name} settings`)
const crumbs = computed<Crumb[]>(() =>
  type.value
    ? [
        { label: 'Assets', to: '/assets' },
        { label: type.value.name, to: typeListLocation(type.value.id, listContext.views) },
        { label: 'Settings' },
      ]
    : [],
)
</script>

<template>
  <section v-if="type">
    <AppBreadcrumb :items="crumbs" />
    <div class="page-header">
      <h1>{{ type.name }} settings <small>({{ type.code }})</small></h1>
      <div class="actions">
        <Tag v-if="type.is_system" value="built-in" severity="secondary" />
        <Tag v-if="type.archived_at" value="archived" severity="secondary" />
      </div>
    </div>

    <form class="form" @submit.prevent="saveDetails">
      <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>
      <div class="field">
        <label for="type-name">Name</label>
        <InputText id="type-name" v-model="name" :disabled="!canManage" />
        <small v-if="errors.fields.value.name" class="field-error">{{ errors.fields.value.name }}</small>
      </div>
      <div class="field">
        <label for="type-desc">Description</label>
        <Textarea id="type-desc" v-model="description" rows="3" :disabled="!canManage" />
      </div>
      <div v-if="canManage" class="actions">
        <Button type="submit" label="Save" :loading="update.isPending.value" />
        <Button
          v-if="!type.is_system"
          :label="type.archived_at ? 'Restore' : 'Archive'"
          severity="secondary"
          text
          @click="toggleArchived"
        />
      </div>
    </form>

    <section>
      <div class="page-header">
        <h2>Attributes</h2>
        <Button v-if="canManage" label="Add attribute" icon="pi pi-plus" size="small" @click="openAttribute(null)" />
      </div>
      <div class="toolbar">
        <Checkbox v-model="showRemoved" input-id="show-removed" binary />
        <label for="show-removed">Show removed</label>
      </div>
      <p v-if="canManage && attributes.length > 1" class="hint">Drag the handle to change the order of columns and form fields.</p>
      <DataTable :value="attributes" data-key="id" @row-reorder="onReorder">
        <Column v-if="canManage && !showRemoved" row-reorder header-style="width: 2.5rem" />
        <Column field="label" header="Label" />
        <Column header="Key">
          <template #body="{ data: a }: { data: Attribute }"><code>{{ a.key }}</code></template>
        </Column>
        <Column field="data_type" header="Type" />
        <Column header="Unit">
          <template #body="{ data: a }: { data: Attribute }">{{ a.unit ?? '' }}</template>
        </Column>
        <Column header="Required">
          <template #body="{ data: a }: { data: Attribute }">{{ a.is_required ? 'Yes' : '' }}</template>
        </Column>
        <Column header="">
          <template #body="{ data: a }: { data: Attribute }">
            <Tag v-if="a.removed" value="removed" severity="secondary" />
            <div v-else class="actions">
              <Button
                v-if="a.data_type === 'select'"
                :label="`Options (${a.options.filter((o) => !o.removed).length})`"
                size="small"
                text
                @click="openOptions(a)"
              />
              <template v-if="canManage">
                <Button label="Edit" size="small" text @click="openAttribute(a)" />
                <Button label="Remove" size="small" text severity="danger" @click="askRemove(a)" />
              </template>
            </div>
          </template>
        </Column>
        <template #empty>No attributes yet.</template>
      </DataTable>
    </section>

    <AttributeDialog v-model:visible="attrOpen" :type-id="type.id" :attribute="attrEditing" :next-position="nextPosition" />
    <OptionsDialog v-model:visible="optionsOpen" :type-id="type.id" :attribute="optionsAttr" :can-manage="canManage" />
  </section>
</template>

<style scoped>
.hint {
  margin: 0 0 0.5rem;
  color: var(--p-text-muted-color);
  font-size: 0.875rem;
}
</style>
