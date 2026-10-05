<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Column from 'primevue/column'
import ContextMenu from 'primevue/contextmenu'
import DataTable, { type DataTableRowReorderEvent } from 'primevue/datatable'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Panel from 'primevue/panel'
import Tag from 'primevue/tag'
import Textarea from 'primevue/textarea'
import { useConfirm } from 'primevue/useconfirm'
import { computed, ref, watch } from 'vue'
import { useTabTitle } from '@/app/tabs/tabPage'
import AppBreadcrumb, { type Crumb } from '@/components/AppBreadcrumb.vue'
import DetailHeader from '@/components/DetailHeader.vue'
import IconAction from '@/components/IconAction.vue'
import type { Attribute } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { describeError, isApiError } from '@/lib/errors'
import { useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { onRowClick, useRowMenu } from '@/lib/tableRows'
import {
  useArchiveAssetType,
  useAssetType,
  useRemoveAttribute,
  useReorderAttributes,
  useRestoreAssetType,
  useUpdateAssetType,
  useUpdateAttribute,
} from '../api'
import { codeMark } from '../code'
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
// Bật/tắt "Required" ngay trong bảng
const updateAttr = useUpdateAttribute()
const toggling = ref<string | null>(null)
async function setRequired(a: Attribute, required: boolean) {
  toggling.value = a.id
  try {
    await updateAttr.mutateAsync({ typeId: props.typeId, attrId: a.id, is_required: required })
  } catch (err) {
    notify.error(describeError(err))
  } finally {
    toggling.value = null
  }
}
const activeOptions = (a: Attribute) =>
  a.options
    .filter((o) => !o.removed)
    .sort((x, y) => x.position - y.position)
    .map((o) => o.label)
    .join(', ')

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

// Bấm dòng để sửa thuộc tính (người được quản lý, thuộc tính chưa xoá); chuột phải có thêm
// sửa lựa chọn và xoá
function canEditRow(a: Attribute) {
  return canManage.value && !a.removed
}
const rowClick = onRowClick((a: Attribute) => canEditRow(a) && openAttribute(a))
const menu = ref<InstanceType<typeof ContextMenu>>()
const { items: menuItems, show: showMenu, clear: clearMenu } = useRowMenu<Attribute>(menu, (a) =>
  canEditRow(a)
    ? [
        { label: 'Edit', icon: 'pi pi-pencil', command: () => openAttribute(a) },
        { label: 'Edit options', icon: 'pi pi-list', visible: a.data_type === 'select', command: () => openOptions(a) },
        { separator: true },
        { label: 'Remove', icon: 'pi pi-trash', command: () => askRemove(a) },
      ]
    : [],
)
</script>

<template>
  <section v-if="type">
    <AppBreadcrumb :items="crumbs" />
    <DetailHeader :title="type.name">
      <template #media>
        <span class="type-mark">{{ codeMark(type.code) }}</span>
      </template>
      <template #tags>
        <Tag v-if="type.is_system" value="Built-in" icon="pi pi-lock" severity="secondary" />
        <Tag v-if="type.archived_at" value="Archived" icon="pi pi-inbox" severity="secondary" />
      </template>
      <div><code>{{ type.code }}</code> · Type settings</div>
    </DetailHeader>

    <!-- Cài đặt chia khối như demo: Chung, Thuộc tính, Lưu trữ -->
    <Panel header="General" class="block">
      <form class="form" @submit.prevent="saveDetails">
        <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>
        <div class="field-row">
          <div class="field grow">
            <label for="type-name">Name</label>
            <InputText id="type-name" v-model="name" :disabled="!canManage" />
            <small v-if="errors.fields.value.name" class="field-error">{{ errors.fields.value.name }}</small>
          </div>
          <div class="field">
            <label for="type-code">Code</label>
            <InputText id="type-code" :model-value="type.code" disabled class="mono" />
          </div>
        </div>
        <div class="field">
          <label for="type-desc">Description</label>
          <Textarea id="type-desc" v-model="description" rows="2" auto-resize :disabled="!canManage" />
        </div>
        <div v-if="canManage" class="actions">
          <Button type="submit" label="Save" :loading="update.isPending.value" />
        </div>
      </form>
    </Panel>

    <Panel header="Attributes" class="block">
      <template #icons>
        <Button v-if="canManage" label="Add attribute" icon="pi pi-plus" size="small" @click="openAttribute(null)" />
      </template>
      <div class="toolbar">
        <Checkbox v-model="showRemoved" input-id="show-removed" binary />
        <label for="show-removed">Show removed</label>
      </div>
      <p v-if="canManage && attributes.length > 1" class="hint">Drag the handle to change the order of columns and form fields.</p>
      <!-- Cột trải hết bề ngang; "Required" là checkbox (đổi ngay khi có quyền) -->
      <ContextMenu ref="menu" :model="menuItems" @hide="clearMenu" />
      <DataTable
        :value="attributes"
        data-key="id"
        table-style="width: 100%; table-layout: fixed"
        row-hover
        :row-class="(a: Attribute) => (canEditRow(a) ? 'clickable-row' : undefined)"
        @row-reorder="onReorder"
        @row-click="rowClick"
        @row-contextmenu="showMenu"
      >
        <Column v-if="canManage && !showRemoved" row-reorder header-style="width: 2.75rem" />
        <Column field="label" header="Label" header-style="width: 22%" />
        <Column header="Key" header-style="width: 16%">
          <template #body="{ data: a }: { data: Attribute }"><code>{{ a.key }}</code></template>
        </Column>
        <Column field="data_type" header="Type" header-style="width: 10%" />
        <Column header="Unit" header-style="width: 9%">
          <template #body="{ data: a }: { data: Attribute }">{{ a.unit ?? '' }}</template>
        </Column>
        <Column header="Required" header-style="width: 9%" body-class="center" header-class="center">
          <template #body="{ data: a }: { data: Attribute }">
            <Checkbox
              :model-value="a.is_required"
              binary
              :disabled="!canManage || a.removed || toggling === a.id"
              :aria-label="`${a.label} is required`"
              @update:model-value="(v: boolean) => setRequired(a, v)"
            />
          </template>
        </Column>
        <Column header="Options" header-style="width: 18%">
          <template #body="{ data: a }: { data: Attribute }">
            <div v-if="a.data_type === 'select'" class="opts-cell">
              <span class="opts">{{ activeOptions(a) || 'No options yet' }}</span>
              <IconAction
                :icon="canManage ? 'pi pi-pencil' : 'pi pi-eye'"
                :label="canManage ? 'Edit options' : 'View options'"
                class="opts-btn"
                @click="openOptions(a)"
              />
            </div>
          </template>
        </Column>
        <Column header="" header-style="width: 6rem" body-class="row-actions-cell">
          <template #body="{ data: a }: { data: Attribute }">
            <Tag v-if="a.removed" value="removed" severity="secondary" />
            <div v-else-if="canManage" class="row-actions">
              <IconAction icon="pi pi-pencil" label="Edit" @click="openAttribute(a)" />
              <IconAction icon="pi pi-trash" label="Remove" danger @click="askRemove(a)" />
            </div>
          </template>
        </Column>
        <template #empty>No attributes yet.</template>
      </DataTable>
      <p class="hint after">New attributes appear as columns and filters in this type's asset list right away.</p>
    </Panel>

    <Panel v-if="canManage && !type.is_system" :header="type.archived_at ? 'Restore this type' : 'Archive this type'" class="block">
      <p class="hint">Archived types can't be chosen for new assets; existing assets keep them.</p>
      <Button
        :label="type.archived_at ? 'Restore' : 'Archive'"
        :icon="type.archived_at ? 'pi pi-replay' : 'pi pi-inbox'"
        :severity="type.archived_at ? 'secondary' : 'danger'"
        outlined
        @click="toggleArchived"
      />
    </Panel>

    <AttributeDialog v-model:visible="attrOpen" :type-id="type.id" :attribute="attrEditing" :next-position="nextPosition" />
    <OptionsDialog v-model:visible="optionsOpen" :type-id="type.id" :attribute="optionsAttr" :can-manage="canManage" />
  </section>
</template>

<style scoped>
.type-mark {
  flex: none;
  display: grid;
  place-items: center;
  width: 3rem;
  height: 3rem;
  border-radius: 10px;
  background: var(--app-soft);
  font: 700 0.9rem var(--app-mono);
  color: var(--p-text-color);
}
.block + .block {
  margin-top: 1rem;
}
.field-row {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
}
.field-row .grow {
  flex: 1;
  min-width: 12rem;
}
:deep(.center) {
  text-align: center;
}
:deep(.center .p-datatable-column-header-content) {
  justify-content: center;
}
/* Nhãn các lựa chọn cắt bằng "…", nút sửa luôn nằm cùng dòng bên phải */
.opts-cell {
  display: flex;
  align-items: center;
  gap: 0.25rem;
}
.opts {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.opts-btn {
  flex: none;
}
.hint.after {
  margin: 0.75rem 0 0;
}
.hint {
  margin: 0 0 0.5rem;
  color: var(--p-text-muted-color);
  font-size: 0.875rem;
}
</style>
