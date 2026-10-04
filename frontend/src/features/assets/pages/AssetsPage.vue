<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Column from 'primevue/column'
import ContextMenu from 'primevue/contextmenu'
import DataTable, {
  type DataTablePageEvent,
  type DataTableRowClickEvent,
  type DataTableRowContextMenuEvent,
  type DataTableSortEvent,
} from 'primevue/datatable'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Tag from 'primevue/tag'
import type { MenuItem } from 'primevue/menuitem'
import { computed, ref, watch } from 'vue'
import type { AssetListItem } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { formatDate, formatDateTime } from '@/lib/dates'
import { PAGE_SIZES, usePageSize } from '@/lib/preferences'
import { useUrlState } from '@/lib/urlState'
import { useAssetType, useAssetTypes } from '@/features/asset-types/api'
import { kindSeverity, statusKinds, useStatuses } from '@/features/statuses/api'
import { useAssetList } from '../api'
import AttributeFilters from '../components/AttributeFilters.vue'
import RetireDialog from '../components/RetireDialog.vue'
import { useListContext } from '../listContext'
import {
  type AttrFilterRow,
  fromTableSort,
  parseAssetQuery,
  serializeAssetQuery,
  switchType,
  toApiParams,
  toTableSort,
} from '../listQuery'
import { useAssetActions } from '../useAssetActions'
import { formatValue } from '../values'

const session = useSession()
const canManage = computed(() => session.can(Perm.AssetManage))
const listContext = useListContext()
const actions = useAssetActions()

const { state, update } = useUrlState(parseAssetQuery, serializeAssetQuery)

// Số dòng mỗi trang: mỗi bảng (mọi loại, từng loại) nhớ số riêng
const { size: pageSize, set: setPageSize } = usePageSize(() => `assets:${state.value.typeId ?? 'all'}`)
const { data, isFetching } = useAssetList(computed(() => toApiParams(state.value, pageSize.value)))

// danh sách lọc gồm cả loại/status đã lưu trữ: tài sản cũ vẫn mang chúng
const { data: types } = useAssetTypes(true)
const { data: statuses } = useStatuses(true)

// Chọn một loại: thêm cột theo thuộc tính đang hoạt động, và bộ lọc theo thuộc tính
const { data: selectedType } = useAssetType(() => state.value.typeId)
const attributes = computed(() =>
  state.value.typeId && selectedType.value
    ? selectedType.value.attributes.filter((a) => !a.removed).sort((a, b) => a.position - b.position)
    : [],
)

// Nhớ danh sách đang xem: trang tài sản dùng để quay lại và bước qua kết quả
watch(
  [data, state, pageSize],
  ([d, s, size]) => {
    if (d) listContext.ctx = { state: s, pageSize: size, ids: d.items.map((a) => a.id), total: d.total }
  },
  { immediate: true },
)

const search = ref(state.value.q)
let timer: ReturnType<typeof setTimeout> | undefined
watch(search, (q) => {
  clearTimeout(timer)
  timer = setTimeout(() => update({ q, page: 1 }), 300)
})

// Đổi loại: bộ lọc thuộc tính, sắp và trang của loại cũ được nhớ, của loại mới được khôi phục
function setType(typeId: string | null) {
  const next = switchType(state.value, typeId ?? undefined, listContext.views)
  listContext.views = next.views
  update(next.state)
}

function applyFilters(filters: AttrFilterRow[]) {
  update({ filters, page: 1 })
}

const tableSort = computed(() => toTableSort(state.value.sort))

function onSort(e: DataTableSortEvent) {
  const field = typeof e.sortField === 'string' ? e.sortField : undefined
  update({ sort: fromTableSort(field, e.sortOrder), page: 1 })
}

function onPage(e: DataTablePageEvent) {
  // đổi số dòng thì về trang 1
  if (e.rows !== pageSize.value) {
    setPageSize(e.rows)
    update({ page: 1 })
    return
  }
  update({ page: e.page + 1 })
}

// Bấm dòng mở tài sản; Ctrl/⌘ mở tab mới. Bấm trúng liên kết hay nút trong dòng thì để chúng tự xử lý
function onRowClick(e: DataTableRowClickEvent) {
  const target = e.originalEvent.target as HTMLElement | null
  if (target?.closest('a, button, input')) return
  actions.open(e.data as AssetListItem, e.originalEvent as MouseEvent)
}

// Menu chuột phải trên dòng
const menu = ref<InstanceType<typeof ContextMenu>>()
const menuRow = ref<AssetListItem | null>(null)
const menuItems = computed<MenuItem[]>(() => {
  const a = menuRow.value
  if (!a) return []
  const items: MenuItem[] = [
    { label: 'Open', icon: 'pi pi-arrow-right', command: () => actions.open(a) },
    { label: 'Open in new tab', icon: 'pi pi-external-link', command: () => actions.open(a, undefined, true) },
  ]
  if (canManage.value) {
    items.push({ separator: true })
    if (a.retired_at) items.push({ label: 'Restore', icon: 'pi pi-replay', command: () => actions.askRestore(a) })
    else {
      items.push({ label: 'Edit', icon: 'pi pi-pencil', command: () => actions.edit(a) })
      items.push({ label: 'Retire', icon: 'pi pi-ban', command: () => actions.askRetire(a) })
    }
  }
  return items
})
function onRowContextMenu(e: DataTableRowContextMenuEvent) {
  menuRow.value = e.data as AssetListItem
  menu.value?.show(e.originalEvent)
}

function cell(row: AssetListItem, key: string) {
  return formatValue(row.attributes?.find((a) => a.key === key))
}

const showFilters = ref(state.value.filters.length > 0)
</script>

<template>
  <section>
    <div class="page-header">
      <h1>Assets</h1>
      <Button v-if="canManage" as="router-link" to="/assets/new" label="New asset" icon="pi pi-plus" />
    </div>

    <div class="toolbar">
      <InputText v-model="search" placeholder="Search tag or name" />
      <Select
        :model-value="state.typeId ?? null"
        :options="types ?? []"
        option-label="name"
        option-value="id"
        placeholder="All types"
        show-clear
        filter
        @update:model-value="setType"
      />
      <Select
        :model-value="state.statusId ?? null"
        :options="statuses ?? []"
        option-label="name"
        option-value="id"
        placeholder="All statuses"
        show-clear
        @update:model-value="(v: string | null) => update({ statusId: v ?? undefined, page: 1 })"
      />
      <Select
        :model-value="state.statusKind ?? null"
        :options="statusKinds"
        placeholder="Any kind"
        show-clear
        @update:model-value="(v) => update({ statusKind: v ?? undefined, page: 1 })"
      />
      <Checkbox
        :model-value="state.includeRetired"
        input-id="include-retired"
        binary
        @update:model-value="(v: boolean) => update({ includeRetired: v, page: 1 })"
      />
      <label for="include-retired">Include retired</label>
      <Button
        v-if="state.typeId"
        :label="`Filters${state.filters.length ? ` (${state.filters.length})` : ''}`"
        icon="pi pi-filter"
        severity="secondary"
        text
        @click="showFilters = !showFilters"
      />
    </div>

    <AttributeFilters
      v-if="state.typeId && showFilters"
      :attributes="attributes"
      :filters="state.filters"
      @apply="applyFilters"
    />

    <ContextMenu ref="menu" :model="menuItems" @hide="menuRow = null" />
    <DataTable
      :value="data?.items ?? []"
      lazy
      paginator
      :rows="pageSize"
      :rows-per-page-options="PAGE_SIZES"
      :first="(state.page - 1) * pageSize"
      :total-records="data?.total ?? 0"
      :loading="isFetching"
      :sort-field="tableSort.field"
      :sort-order="tableSort.order"
      removable-sort
      data-key="id"
      scrollable
      row-hover
      :row-class="() => 'clickable-row'"
      @page="onPage"
      @sort="onSort"
      @row-click="onRowClick"
      @row-contextmenu="onRowContextMenu"
    >
      <Column header="Tag" sort-field="tag" sortable>
        <template #body="{ data: a }: { data: AssetListItem }">
          <RouterLink :to="`/assets/${a.id}`">{{ a.tag }}</RouterLink>
        </template>
      </Column>
      <Column field="name" header="Name" sort-field="name" sortable />
      <Column field="asset_type_name" header="Type" sort-field="asset_type" sortable />
      <Column header="Status" sort-field="status" sortable>
        <template #body="{ data: a }: { data: AssetListItem }">
          <Tag :value="a.status_name" :severity="kindSeverity(a.status_kind)" />
        </template>
      </Column>
      <Column
        v-for="attr in attributes"
        :key="attr.id"
        :header="attr.unit ? `${attr.label} (${attr.unit})` : attr.label"
        :sort-field="`attributes.${attr.key}`"
        sortable
      >
        <template #body="{ data: a }: { data: AssetListItem }">{{ cell(a, attr.key) }}</template>
      </Column>
      <Column header="Purchased" sort-field="purchase_date" sortable>
        <template #body="{ data: a }: { data: AssetListItem }">{{ formatDate(a.purchase_date) }}</template>
      </Column>
      <Column header="Updated" sort-field="updated_at" sortable>
        <template #body="{ data: a }: { data: AssetListItem }">{{ formatDateTime(a.updated_at) }}</template>
      </Column>
      <Column v-if="canManage" header="" class="row-actions-col">
        <template #body="{ data: a }: { data: AssetListItem }">
          <div class="row-actions">
            <template v-if="!a.retired_at">
              <Button label="Edit" size="small" text @click="(e: MouseEvent) => actions.edit(a, e)" />
              <Button label="Retire" size="small" text severity="danger" @click="actions.askRetire(a)" />
            </template>
            <Button v-else label="Restore" size="small" text @click="actions.askRestore(a)" />
          </div>
        </template>
      </Column>
      <template #empty>No assets found.</template>
    </DataTable>

    <RetireDialog v-model:visible="actions.retireOpen.value" :asset="actions.retireTarget.value" />
  </section>
</template>

<style scoped>
:deep(.clickable-row) {
  cursor: pointer;
}
.row-actions {
  display: flex;
  gap: 0.25rem;
  justify-content: flex-end;
  white-space: nowrap;
}
/* Nút hành động hiện khi rê chuột hay focus vào dòng; màn hình cảm ứng luôn hiện */
@media (hover: hover) {
  :deep(.clickable-row) .row-actions {
    opacity: 0;
  }
  :deep(.clickable-row:hover) .row-actions,
  :deep(.clickable-row:focus-within) .row-actions {
    opacity: 1;
  }
}
</style>
