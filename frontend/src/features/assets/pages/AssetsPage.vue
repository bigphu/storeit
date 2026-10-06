<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Chip from 'primevue/chip'
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
import { useRouter } from 'vue-router'
import { useTabId, useTabQuery, useTabTitle } from '@/app/tabs/tabPage'
import type { AssetListItem } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { formatDate, formatDateTime } from '@/lib/dates'
import { usePageKeys } from '@/lib/pageKeys'
import { PAGE_SIZES, usePageSize } from '@/lib/preferences'
import { useAssetType, useAssetTypes } from '@/features/asset-types/api'
import { kindSeverity, statusKinds, useStatuses } from '@/features/statuses/api'
import { useAssetList } from '../api'
import AttributeFilterPopover from '../components/AttributeFilterPopover.vue'
import BulkActionDialog from '../components/BulkActionDialog.vue'
import RetireDialog from '../components/RetireDialog.vue'
import { attrFilterLabel, type FilterChip, filterChips, removeChip } from '../filterChips'
import ExportButton from '../export/components/ExportButton.vue'
import ReportDialog from '../export/components/ReportDialog.vue'
import type { ExportScope } from '../export/usePreviewData'
import { useExport } from '../export/useExport'
import { useListContext } from '../listContext'
import {
  type AssetListState,
  type AttrFilterRow,
  fromTableSort,
  listLocation,
  parseAssetQuery,
  toApiParams,
  toTableSort,
} from '../listQuery'
import { useAssetActions } from '../useAssetActions'
import { formatValue } from '../values'

// typeId (từ /types/:typeId/assets): danh sách của một loại; không có là mọi loại
const props = defineProps<{ typeId?: string }>()

const session = useSession()
// trang được giữ sống khi chuyển tab: chỉ theo URL của tab mình
const tabId = useTabId()
const query = useTabQuery()
const router = useRouter()
const canManage = computed(() => session.can(Perm.AssetManage))
const listContext = useListContext()
const actions = useAssetActions()

// State nằm trên URL: loại ở path, phần còn lại ở query
const state = computed<AssetListState>(() => ({ ...parseAssetQuery(query.value), typeId: props.typeId }))
function update(patch: Partial<AssetListState>) {
  router.replace(listLocation({ ...state.value, ...patch }))
}

// Số dòng mỗi trang: mỗi bảng (mọi loại, từng loại) nhớ số riêng
const { size: pageSize, set: setPageSize } = usePageSize(() => `assets:${state.value.typeId ?? 'all'}`)
const { data, isFetching } = useAssetList(computed(() => toApiParams(state.value, pageSize.value)))

// danh sách lọc gồm cả status đã lưu trữ: tài sản cũ vẫn mang chúng
const { data: statuses } = useStatuses(true)

// Chọn một loại: thêm cột theo thuộc tính đang hoạt động, và bộ lọc theo thuộc tính
const { data: selectedType } = useAssetType(() => state.value.typeId)
const attributes = computed(() =>
  state.value.typeId && selectedType.value
    ? selectedType.value.attributes.filter((a) => !a.removed).sort((a, b) => a.position - b.position)
    : [],
)

// Nhớ danh sách đang xem: trang tài sản dùng để quay lại và bước qua kết quả; mỗi
// loại nhớ bộ lọc thuộc tính, sắp và trang riêng (sidebar, bộ chọn loại mở lại đúng view)
watch(
  [data, state, pageSize],
  ([d, s, size]) => {
    if (d) listContext.setCtx(tabId, { state: s, pageSize: size, ids: d.items.map((a) => a.id), total: d.total })
    listContext.views = { ...listContext.views, [s.typeId ?? '']: { filters: s.filters, sort: s.sort, page: s.page } }
  },
  { immediate: true },
)

const search = ref(state.value.q)
let timer: ReturnType<typeof setTimeout> | undefined
watch(search, (q) => {
  clearTimeout(timer)
  timer = setTimeout(() => update({ q, page: 1 }), 300)
})
// đổi loại (sidebar, bộ chọn loại) có thể mang tìm kiếm khác: ô tìm kiếm theo URL
watch(
  () => state.value.q,
  (q) => {
    if (q !== search.value) search.value = q
  },
)

// Bộ lọc đang áp dụng thành chip; ✕ bỏ từng cái, "Clear all" bỏ hết
const chips = computed(() => filterChips(state.value, attributes.value, statuses.value ?? []))
const chipKey = (c: FilterChip) => `${c.kind}:${c.label}`
function clearAll() {
  update({ q: '', statusId: undefined, statusKind: undefined, includeRetired: false, filters: [], page: 1 })
}
const filterPop = ref<InstanceType<typeof AttributeFilterPopover>>()
function addFilter(f: AttrFilterRow) {
  update({ filters: [...state.value.filters, f], page: 1 })
}

// "48 of 312 assets": tổng của phạm vi (chưa retire) lấy từ số đếm theo loại
const { data: typeCounts } = useAssetTypes(false, true)
const countText = computed(() => {
  const total = data.value?.total
  if (total === undefined) return ''
  const counts = typeCounts.value ?? []
  const scope = props.typeId
    ? counts.find((t) => t.id === props.typeId)?.asset_count
    : counts.reduce((n, t) => n + (t.asset_count ?? 0), 0)
  const noun = total === 1 ? 'asset' : 'assets'
  return chips.value.length && !state.value.includeRetired && scope !== undefined
    ? `${total} of ${scope} ${noun}`
    : `${total} ${noun}`
})

// Phím: / tìm kiếm, N tạo tài sản (của loại đang xem)
const newPath = computed(() => (props.typeId ? `/types/${props.typeId}/assets/new` : '/assets/new'))
usePageKeys((e) => {
  if (e.key === '/') {
    e.preventDefault()
    document.getElementById('asset-search')?.focus()
  } else if (e.key === 'n' && canManage.value) {
    router.push(newPath.value)
  }
})

// Phạm vi export: bộ lọc của danh sách (không phân trang), hay các dòng đang chọn.
// Tên khoá của toApiParams trùng ExportFilters nên chỉ cần bỏ page/page_size
const canExport = computed(() => session.can(Perm.AssetExport))
const reportOpen = ref(false)
const reportProfile = ref<string | undefined>()
const reportSelection = ref(false)
const listFilters = computed(() => {
  const { page: _p, page_size: _s, ...filters } = toApiParams(state.value, pageSize.value)
  return filters
})
const listLabel = computed(() => {
  const name = selectedType.value?.name ?? 'All assets'
  return chips.value.length ? [name, ...chips.value.map((c) => c.label)].join(' · ') : name
})
const listScope = computed<ExportScope>(() => ({
  filters: listFilters.value,
  label: listLabel.value,
  count: data.value?.total ?? 0,
  typeIds: state.value.typeId
    ? [state.value.typeId]
    : (typeCounts.value ?? []).filter((t) => (t.asset_count ?? 0) > 0).map((t) => t.id),
  rows: data.value?.items ?? [],
  selection: false,
}))
const selectionScope = computed<ExportScope>(() => {
  const ids = selected.value.map((a) => a.id)
  return {
    filters: { ...listFilters.value, ids },
    label: `${ids.length} selected assets`,
    count: ids.length,
    typeIds: [...new Set(selected.value.map((a) => a.asset_type_id))],
    rows: selected.value,
    selection: true,
  }
})
const exportScope = computed(() => (reportSelection.value ? selectionScope.value : listScope.value))
function openReport(selection: boolean, profileId?: string) {
  reportSelection.value = selection
  reportProfile.value = profileId
  reportOpen.value = true
}
const { run: runExport } = useExport()

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

// Chọn nhiều dòng để đổi status hay retire một lần; đổi bộ lọc hay loại thì bỏ chọn
const selected = ref<AssetListItem[]>([])
watch(
  () => JSON.stringify({ ...state.value, page: 0, sort: '' }),
  () => (selected.value = []),
)
const bulkOpen = ref(false)
const bulkMode = ref<'status' | 'retire'>('status')
function openBulk(mode: 'status' | 'retire') {
  bulkMode.value = mode
  bulkOpen.value = true
}

// Bấm dòng mở tài sản; Ctrl/⌘ mở tab mới. Bấm trúng liên kết, nút hay ô chọn thì để chúng tự xử lý
function onRowClick(e: DataTableRowClickEvent) {
  const target = e.originalEvent.target as HTMLElement | null
  if (target?.closest('a, button, input, .p-checkbox, .select-cell')) return
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

// Tiêu đề tab: tên loại và bộ lọc thuộc tính đầu tiên ("Laptop · RAM ≥ 16 GB +1")
useTabTitle(() => {
  if (!props.typeId) return state.value.statusKind ? `All assets · ${state.value.statusKind.replace('_', ' ')}` : 'All assets'
  const name = selectedType.value?.name
  if (!name) return undefined
  const [f, ...more] = state.value.filters
  if (!f) return name
  return `${name} · ${attrFilterLabel(f, attributes.value)}${more.length ? ` +${more.length}` : ''}`
})

function cell(row: AssetListItem, key: string) {
  return formatValue(row.attributes?.find((a) => a.key === key))
}

</script>

<template>
  <section>
    <div class="page-header">
      <h1>{{ typeId ? (selectedType?.name ?? '') : 'All assets' }}</h1>
      <div class="actions">
        <Button
          v-if="typeId"
          as="router-link"
          :to="`/types/${typeId}/settings`"
          label="Type settings"
          style="text-decoration: none"
          icon="pi pi-cog"
          severity="secondary"
          outlined
        />
        <Button
          v-if="canManage"
          as="router-link"
          :to="newPath"
          :label="selectedType ? `New ${selectedType.name.toLowerCase()}` : 'New asset'"
          style="text-decoration: none"
          icon="pi pi-plus"
        />
      </div>
    </div>

    <div class="toolbar">
      <InputText id="asset-search" v-model="search" placeholder="Search tag or name  ( / )" aria-label="Search tag or name" />
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
      <ExportButton v-if="canExport" class="export-btn" :scope="listScope" @report="(id) => openReport(false, id)" />
    </div>

    <div class="chips">
      <Chip
        v-for="c in chips"
        :key="chipKey(c)"
        :label="c.label"
        removable
        :class="{ 'chip-attr': c.kind === 'attr' }"
        @remove="update(removeChip(state, c))"
      />
      <Button
        v-if="typeId"
        label="Filter"
        icon="pi pi-plus"
        size="small"
        text
        :disabled="state.filters.length >= 10 || !attributes.length"
        :title="attributes.length ? 'Filter by an attribute of this type' : 'This type has no attributes yet'"
        @click="(e: MouseEvent) => filterPop?.toggle(e)"
      />
      <span v-else class="hint">Open a type to filter by its attributes.</span>
      <Button v-if="chips.length" label="Clear all" size="small" text severity="secondary" @click="clearAll" />
      <span class="count">{{ countText }}</span>
    </div>
    <AttributeFilterPopover ref="filterPop" :attributes="attributes" @add="addFilter" />

    <div v-if="selected.length" class="selection-bar" role="region" aria-label="Selected assets">
      <span class="selection-count">{{ selected.length }} selected</span>
      <Button label="Change status" icon="pi pi-tag" size="small" @click="openBulk('status')" />
      <Button label="Retire" icon="pi pi-ban" size="small" severity="danger" outlined @click="openBulk('retire')" />
      <Button
        v-if="canExport"
        label="Export selected"
        icon="pi pi-download"
        size="small"
        outlined
        @click="runExport({ mode: 'data', filters: selectionScope.filters }, 'storeit-assets.xlsx')"
      />
      <Button v-if="canExport" label="Report from selected…" icon="pi pi-file-edit" size="small" outlined @click="openReport(true)" />
      <Button label="Clear selection" size="small" text severity="secondary" @click="selected = []" />
    </div>
    <BulkActionDialog v-model:visible="bulkOpen" :mode="bulkMode" :rows="selected" @done="selected = []" />
    <ReportDialog v-if="canExport" v-model:visible="reportOpen" :scope="exportScope" :profile-id="reportProfile" />

    <ContextMenu ref="menu" :model="menuItems" @hide="menuRow = null" />
    <DataTable
      :value="data?.items ?? []"
      lazy
      paginator
      :rows="pageSize"
      :rows-per-page-options="PAGE_SIZES"
      paginator-template="FirstPageLink PrevPageLink PageLinks NextPageLink LastPageLink CurrentPageReport RowsPerPageDropdown"
      current-page-report-template="Showing {first}–{last} of {totalRecords}"
      :first="(state.page - 1) * pageSize"
      :total-records="data?.total ?? 0"
      :loading="isFetching"
      :sort-field="tableSort.field"
      :sort-order="tableSort.order"
      removable-sort
      v-model:selection="selected"
      data-key="id"
      scrollable
      row-hover
      :row-class="() => 'clickable-row'"
      @page="onPage"
      @sort="onSort"
      @row-click="onRowClick"
      @row-contextmenu="onRowContextMenu"
    >
      <Column v-if="canManage" selection-mode="multiple" header-style="width: 3rem" body-class="select-cell" />
      <Column header="Tag" sort-field="tag" sortable body-class="tag-cell">
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
              <Button
                v-tooltip.top="'Edit'"
                icon="pi pi-pencil"
                size="small"
                text
                rounded
                aria-label="Edit"
                @click="(e: MouseEvent) => actions.edit(a, e)"
              />
              <Button
                v-tooltip.top="'Retire'"
                icon="pi pi-ban"
                size="small"
                text
                rounded
                severity="danger"
                aria-label="Retire"
                @click="actions.askRetire(a)"
              />
            </template>
            <Button
              v-else
              v-tooltip.top="'Restore'"
              icon="pi pi-replay"
              size="small"
              text
              rounded
              aria-label="Restore"
              @click="actions.askRestore(a)"
            />
          </div>
        </template>
      </Column>
      <template #empty>No assets found.</template>
    </DataTable>

    <RetireDialog v-model:visible="actions.retireOpen.value" :asset="actions.retireTarget.value" />
  </section>
</template>

<style scoped>
.selection-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  margin-bottom: 0.75rem;
  border-radius: var(--p-content-border-radius);
  background: var(--p-highlight-background);
  color: var(--p-highlight-color);
}
.selection-count {
  font-weight: 600;
  margin-right: 0.5rem;
}
.export-btn {
  margin-left: auto;
}
.chips {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem;
  margin-bottom: 0.75rem;
  min-height: 2rem;
}
.chip-attr {
  background: var(--p-highlight-background);
  color: var(--p-highlight-color);
}
.hint {
  color: var(--p-text-muted-color);
  font-size: 0.875rem;
}
.count {
  margin-left: auto;
  color: var(--p-text-muted-color);
  font-size: 0.875rem;
  font-variant-numeric: tabular-nums;
}
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
