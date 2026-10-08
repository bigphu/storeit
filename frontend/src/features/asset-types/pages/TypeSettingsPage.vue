<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Column from 'primevue/column'
import DataTable, { type DataTableRowReorderEvent } from 'primevue/datatable'
import Tab from 'primevue/tab'
import TabList from 'primevue/tablist'
import Tabs from 'primevue/tabs'
import Tag from 'primevue/tag'
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useLeaveGuard, useTabDirty, useTabTitle } from '@/app/tabs/tabPage'
import AppBreadcrumb, { type Crumb } from '@/components/AppBreadcrumb.vue'
import DetailHeader from '@/components/DetailHeader.vue'
import EmptyState from '@/components/EmptyState.vue'
import OverviewFields, { type FieldDef } from '@/components/OverviewFields.vue'
import SaveBar from '@/components/SaveBar.vue'
import TableSkeleton from '@/components/TableSkeleton.vue'
import IconAction from '@/components/IconAction.vue'
import RowMenuButton from '@/components/RowMenuButton.vue'
import RowMenus from '@/components/RowMenus.vue'
import type { Attribute } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { runAction } from '@/lib/actions'
import { changeCount, changesOf, clearTab, discardTab, emptyDraft, isDirty, restoreTab } from '@/lib/detailDraft'
import { describeError } from '@/lib/errors'
import { notify } from '@/lib/notify'
import { useListTable, useRowMenu } from '@/lib/tableRows'
import { useUrlState } from '@/lib/urlState'
import { useAssetType, useAssetTypes, useRemoveAttribute, useReorderAttributes, useRestoreAttribute, useUpdateAttribute } from '../api'
import { useTypeArchive, useTypeOverviewSave } from '../overviewSave'
import AttributeDialog from '../components/AttributeDialog.vue'
import OptionsDialog from '../components/OptionsDialog.vue'
import { useListContext } from '@/features/assets/listContext'
import { typeListLocation } from '@/features/assets/listQuery'

// Trang chi tiết của một loại: Overview (tên, mã, mô tả sửa tại chỗ) và Attributes; archive
// ở đầu trang
const props = defineProps<{ typeId: string }>()

const session = useSession()
const canManage = computed(() => session.can(Perm.TypeManage))

const { data: type, isLoading } = useAssetType(() => props.typeId)
// số tài sản của loại (dòng thông tin ở đầu trang)
const { data: allTypes } = useAssetTypes(true, true)
const assetCount = computed(() => allTypes.value?.find((t) => t.id === props.typeId)?.asset_count ?? 0)
const activeAttributes = computed(() => (type.value?.attributes ?? []).filter((a) => !a.removed).length)

type Section = 'overview' | 'attributes'
const { state, update: updateUrl } = useUrlState(
  (q) => ({ tab: (q.tab === 'attributes' ? 'attributes' : 'overview') as Section }),
  (s) => ({ tab: s.tab === 'attributes' ? s.tab : undefined }),
)

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
  const activeIds = (list: Attribute[]) => list.filter((a) => !a.removed).map((a) => a.id)
  const before = activeIds(attributes.value)
  attributes.value = e.value as Attribute[]
  const after = activeIds(attributes.value)
  void runAction({
    run: () => reorder.mutateAsync({ typeId: props.typeId, ids: after }),
    done: 'Attributes reordered.',
    failed: "Couldn't save the new order.",
    undo: () => reorder.mutateAsync({ typeId: props.typeId, ids: before }),
    undone: 'Order put back.',
    undoFailed: "Couldn't put the order back.",
  })
}
const nextPosition = computed(() => Math.max(0, ...(type.value?.attributes ?? []).map((a) => a.position)) + 1)

// Overview: sửa tại chỗ, gom vào thanh lưu (Ctrl/⌘ S); Save và Discard đều có Undo
const fields: FieldDef[] = [
  { key: 'name', label: 'Name', maxlength: 100 },
  { key: 'code', label: 'Code', lock: 'Part of every asset tag, so it can’t change.' },
  { key: 'description', label: 'Description', kind: 'textarea' },
]
const saved = computed(() => ({ name: type.value?.name ?? '', code: type.value?.code ?? '', description: type.value?.description ?? '' }))
const draft = reactive(emptyDraft())
useTabDirty(() => isDirty(draft))
useLeaveGuard(() => isDirty(draft))
const saveOverview = useTypeOverviewSave()
const saving = ref(false)
async function save() {
  const t = type.value
  if (!t) return
  saving.value = true
  try {
    if (await saveOverview(t, changesOf(draft, 'overview'))) clearTab(draft, 'overview')
  } finally {
    saving.value = false
  }
}
function discard() {
  const removed = discardTab(draft, 'overview')
  notify.success('Changes discarded.', { undo: () => restoreTab(draft, 'overview', removed) })
}

const toggleArchive = useTypeArchive()
function toggleArchived() {
  if (type.value) void toggleArchive(type.value)
}

// Loại vừa tạo: mở ở Attributes, nút "Add attribute" sẵn focus
const route = useRoute()
watch(
  () => [type.value?.id, state.value.tab] as const,
  async ([id, tab]) => {
    if (!id || tab !== 'attributes' || route.query.new !== '1') return
    await nextTick()
    document.getElementById('add-attribute')?.focus()
  },
  { immediate: true },
)

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
const restoreAttr = useRestoreAttribute()
function removeAttribute(a: Attribute) {
  const ids = { typeId: props.typeId, attrId: a.id }
  return runAction({
    run: () => removeAttr.mutateAsync(ids),
    done: `${a.label} removed.`,
    failed: `Couldn't remove ${a.label}.`,
    undo: () => restoreAttr.mutateAsync(ids),
    undone: `${a.label} is back.`,
    undoFailed: `Couldn't bring ${a.label} back. It stays removed.`,
  })
}

const listContext = useListContext()
useTabTitle(() => type.value?.name)
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
// menu của dòng: chuột phải và nút ☰ cuối dòng
const rowMenu = useRowMenu<Attribute>((a) =>
  canEditRow(a)
    ? [
        { label: 'Edit', icon: 'pi pi-pencil', command: () => openAttribute(a) },
        { label: 'Edit options', icon: 'pi pi-list', visible: a.data_type === 'select', command: () => openOptions(a) },
        { separator: true },
        { label: 'Remove', icon: 'pi pi-trash', command: () => removeAttribute(a) },
      ]
    : [],
)
// bảng: bấm dòng mở, menu chuột phải, nút cuối dòng cho hàng đang dùng (lib/tableRows.ts)
const table = useListTable<Attribute>({ open: (a) => openAttribute(a), clickable: canEditRow, showMenu: rowMenu.showContext })
</script>

<template>
  <section v-if="type">
    <AppBreadcrumb :items="crumbs" />
    <DetailHeader :title="type.name" icon="sitemap">
      <template #tags>
        <Tag :value="type.archived_at ? 'Archived' : 'Active'" :severity="type.archived_at ? 'secondary' : 'success'" />
        <Tag v-if="type.is_system" value="Built-in" icon="pi pi-lock" severity="secondary" />
      </template>
      <div>Code <code>{{ type.code }}</code> · {{ assetCount }} {{ assetCount === 1 ? 'asset' : 'assets' }} · {{ activeAttributes }} {{ activeAttributes === 1 ? 'attribute' : 'attributes' }}</div>
      <template #actions>
        <Button v-slot="slot" as-child text>
          <RouterLink :to="typeListLocation(type.id, listContext.views)" :class="slot.class">View {{ assetCount }} {{ assetCount === 1 ? 'asset' : 'assets' }} <i class="pi pi-arrow-right" aria-hidden="true" /></RouterLink>
        </Button>
        <Button v-if="canManage && !type.is_system" :label="type.archived_at ? 'Restore' : 'Archive'" severity="secondary" outlined @click="toggleArchived" />
      </template>
    </DetailHeader>

    <Tabs :value="state.tab" class="section-tabs" @update:value="(v) => updateUrl({ tab: v as Section })">
      <TabList>
        <Tab value="overview">Overview<span v-if="changeCount(draft, 'overview')" class="tab-dirty" aria-label="Unsaved changes" /></Tab>
        <Tab value="attributes">Attributes <span class="tab-count">{{ activeAttributes }}</span></Tab>
      </TabList>
    </Tabs>

    <template v-if="state.tab === 'overview'">
      <OverviewFields :fields="fields" :saved="saved" :draft="draft" :readonly="!canManage" />
      <SaveBar v-if="canManage && changeCount(draft, 'overview')" :count="changeCount(draft, 'overview')" :saving="saving" @save="save" @discard="discard" />
    </template>

    <template v-else>
      <div class="attr-head">
        <p class="hint">New attributes appear as columns and filters in this type's asset list right away.</p>
        <Button v-if="canManage" id="add-attribute" label="Add attribute" icon="pi pi-plus" size="small" @click="openAttribute(null)" />
      </div>
    <div class="toolbar">
      <Checkbox v-model="showRemoved" input-id="show-removed" binary />
      <label for="show-removed">Show removed</label>
    </div>
    <p v-if="canManage && attributes.length > 1" class="hint">Drag the handle to change the order of columns and form fields.</p>
    <!-- Mỗi dòng một dòng chữ, cột giãn theo nội dung (bảng dài thì cuộn ngang);
         "Required" là checkbox (đổi ngay khi có quyền) -->
    <RowMenus :menu="rowMenu" />
    <DataTable
      :value="attributes"
      data-key="id"
      scrollable
      v-bind="table.bind"
      @row-reorder="onReorder"
    >
      <Column v-if="canManage && !showRemoved" row-reorder row-reorder-icon="pi pi-arrows-v" header-style="width: 2.75rem" />
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
      <Column header="">
        <template #body="{ data: a }: { data: Attribute }">
          <Tag v-if="a.removed" value="removed" severity="secondary" />
        </template>
      </Column>
      <!-- nút ☰ luôn ở mép phải của bảng, kể cả khi bảng cuộn ngang -->
      <Column frozen align-frozen="right" header-class="row-menu-col" body-class="row-menu-col">
        <template #body="{ data: a, index }: { data: Attribute; index: number }">
          <RowMenuButton :active="table.active.isActive(index) && canEditRow(a)" @open="(e) => rowMenu.toggle(a, e)" />
        </template>
      </Column>
      <template #empty>
        <TableSkeleton v-if="isLoading" />
        <EmptyState v-else icon="pi pi-tag" text="No attributes yet." :action="canManage ? 'Add attribute' : undefined" @action="openAttribute(null)" />
      </template>
    </DataTable>
    <p class="hint after">New attributes appear as columns and filters in this type's asset list right away.</p>
    </template>

    <AttributeDialog v-model:visible="attrOpen" :type-id="type.id" :attribute="attrEditing" :next-position="nextPosition" />
    <OptionsDialog v-model:visible="optionsOpen" :type-id="type.id" :attribute="optionsAttr" :can-manage="canManage" />
  </section>
</template>

<style scoped>
.section-tabs {
  margin-bottom: 1rem;
}
.tab-count {
  font: 0.75rem var(--app-mono);
  color: var(--p-text-muted-color);
}
/* tab còn thay đổi chưa lưu */
.tab-dirty {
  display: inline-block;
  width: 0.45rem;
  height: 0.45rem;
  margin-left: 0.4rem;
  border-radius: 50%;
  background: var(--app-warn);
}
.attr-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.5rem;
}
.attr-head .hint {
  margin: 0;
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
/* danh sách lựa chọn dài thì cắt "…" (bấm bút chì để xem hết) */
.opts {
  flex: 1;
  min-width: 0;
  max-width: 18rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.opts-btn {
  flex: none;
}
.hint {
  margin: 0 0 0.5rem;
  color: var(--p-text-muted-color);
  font-size: 0.875rem;
}
</style>
