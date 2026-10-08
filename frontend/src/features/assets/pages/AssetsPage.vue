<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Chip from 'primevue/chip'
import Column from 'primevue/column'
import DataTable, {
  type DataTablePageEvent,
  type DataTableSortEvent,
} from 'primevue/datatable'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Tag from 'primevue/tag'
import type { MenuItem } from 'primevue/menuitem'
import { computed, reactive, ref, watch } from 'vue'
import { loadRouteLocation, useRouter } from 'vue-router'
import { useQueryClient } from '@tanstack/vue-query'
import { useTabId, useTabQuery, useTabTitle } from '@/app/tabs/tabPage'
import type { AssetListItem, DataType } from '@/lib/api/types'
import InlineCell from '@/components/InlineCell.vue'
import RowMenuButton from '@/components/RowMenuButton.vue'
import RowMenus from '@/components/RowMenus.vue'
import OverviewFields, { type FieldDef } from '@/components/OverviewFields.vue'
import QuickEditDrawer from '@/components/QuickEditDrawer.vue'
import { mayClose } from '@/lib/confirm'
import { useQuickDrawer } from '@/lib/quickDrawer'
import { changesOf, clearTab, emptyDraft, isDirty } from '@/lib/detailDraft'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { formatDate, formatDateTime, toDateString } from '@/lib/dates'
import { usePageKeys } from '@/lib/pageKeys'
import { notify } from '@/lib/notify'
import { PAGE_SIZES, usePageSize } from '@/lib/preferences'
import { stepPage, useKeepScrollOnPage, useListTable, useRowMenu } from '@/lib/tableRows'
import { useAssetType, useAssetTypes } from '@/features/asset-types/api'
import { kindSeverity, statusKinds, useStatuses } from '@/features/statuses/api'
import { assetListQuery, assetQuery, useAsset, useAssetList } from '../api'
import EmptyState from '@/components/EmptyState.vue'
import KeyHint from '@/components/KeyHint.vue'
import TableSkeleton from '@/components/TableSkeleton.vue'
import AttributeFilterPopover from '../components/AttributeFilterPopover.vue'
import BulkActionDialog from '../components/BulkActionDialog.vue'
import RetireDialog from '../components/RetireDialog.vue'
import { attrFilterLabel, type FilterChip, filterChips, removeChip } from '../filterChips'
import DataExportDialog from '../export/components/DataExportDialog.vue'
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
  nextPageParams,
  parseAssetQuery,
  toApiParams,
  toTableSort,
} from '../listQuery'
import { useAssetActions } from '../useAssetActions'
import { attrInputError, attrText, type FormValues, formatValue, formValueFrom, fromApiValues } from '../values'

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
const { data, isFetching, isLoading } = useAssetList(computed(() => toApiParams(state.value, pageSize.value)))
// đổi trang (phân trang hay J/K): giữ chỗ đang cuộn tới, dọc lẫn ngang
const tableRef = ref<{ $el: HTMLElement }>()
const keepScroll = useKeepScrollOnPage(() => tableRef.value?.$el, data)

// Tải trước, không đợi người dùng bấm: trang kế tiếp khi trang này về (sang trang tức thì);
// dòng chuột dừng trên đó một lúc thì tải dữ liệu và code của trang tài sản đó (mở nhanh hơn)
const qc = useQueryClient()
// trang kế tiếp đợi lúc trình duyệt rảnh (sau khi bảng vẽ xong): không giành CPU với lần vẽ đầu
const whenIdle = (fn: () => void) =>
  typeof window.requestIdleCallback === 'function' ? window.requestIdleCallback(fn, { timeout: 3000 }) : setTimeout(fn, 1000)
watch(data, (d) => {
  const next = d && nextPageParams(state.value, pageSize.value, d.total)
  if (next) whenIdle(() => void qc.prefetchQuery({ ...assetListQuery(next), meta: { toast: false } }))
})
let hoverTimer: ReturnType<typeof setTimeout> | undefined
function prefetchHovered(i: number | null) {
  clearTimeout(hoverTimer)
  if (i === null) return
  hoverTimer = setTimeout(() => {
    const a = data.value?.items[i]
    if (!a) return
    void qc.prefetchQuery({ ...assetQuery(a.id), meta: { toast: false } })
    loadRouteLocation(router.resolve(`/assets/${a.id}`)).catch(() => {})
  }, 120)
}

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

// Phím: / tìm kiếm, N tạo tài sản (của loại đang xem), J/K trang trước/sau
const newPath = computed(() => (props.typeId ? `/types/${props.typeId}/assets/new` : '/assets/new'))
const newLabel = computed(() => (selectedType.value ? `New ${selectedType.value.name.toLowerCase()}` : 'New asset'))
usePageKeys((e) => {
  if (e.key === '/') {
    e.preventDefault()
    document.getElementById('asset-search')?.focus()
  } else if (e.key === 'n' && canManage.value) {
    router.push(newPath.value)
  } else if (e.key === 'j' || e.key === 'k') {
    const p = stepPage(state.value.page, pageSize.value, data.value?.total ?? 0, e.key === 'j' ? -1 : 1)
    if (p) {
      keepScroll.beforePageChange()
      update({ page: p })
    }
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
    label: ids.length === 1 ? '1 selected asset' : `${ids.length} selected assets`,
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
const { run: runExport, running: exporting } = useExport()
// Export selected: phạm vi chụp lúc bấm, để "What’s inside" vẫn đúng khi đã bỏ chọn
const insideScope = ref<ExportScope | null>(null)
const insideOpen = computed({
  get: () => insideScope.value !== null,
  set: (open) => {
    if (!open) insideScope.value = null
  },
})
function exportSelected() {
  const scope = selectionScope.value
  runExport({ mode: 'data', filters: scope.filters }, 'storeit-assets.xlsx', { rows: scope.count, inside: () => (insideScope.value = scope) })
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
  keepScroll.beforePageChange()
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

// Menu của dòng (chuột phải và nút ☰ cuối dòng): mở, sửa nhanh, sửa, retire / restore
const rowMenu = useRowMenu<AssetListItem>((a) => {
  const items: MenuItem[] = [
    { label: 'Open', icon: 'pi pi-arrow-right', command: () => actions.open(a) },
    { label: 'Open in new tab', icon: 'pi pi-external-link', command: () => actions.open(a, undefined, true) },
  ]
  if (canManage.value) {
    items.push({ separator: true })
    if (a.retired_at) items.push({ label: 'Restore', icon: 'pi pi-replay', command: () => actions.restore(a) })
    else {
      items.push({ label: 'Quick edit', icon: 'pi pi-sliders-h', command: () => openQuick(a) })
      items.push({ label: 'Edit', icon: 'pi pi-pencil', command: () => actions.edit(a) })
      items.push({ label: 'Retire', icon: 'pi pi-ban', command: () => actions.askRetire(a) })
    }
  }
  return items
})

// bảng: bấm dòng mở tài sản (Ctrl/⌘ mở tab mới), menu của dòng, nút ☰ cho hàng đang dùng,
// tải trước tài sản khi chuột dừng trên dòng
const table = useListTable<AssetListItem>({ open: (a, e) => actions.open(a, e), showMenu: rowMenu.showContext, onHover: prefetchHovered })

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


// Sửa nhanh: bút chì cạnh tên, status, ngày mua để đổi tại chỗ; nút thanh trượt (pi-sliders-h) mở ngăn kéo (các trường
// của tài sản và thuộc tính của loại). Tài sản đã retire không sửa nhanh được
const statusChoices = computed(() =>
  (statuses.value ?? []).filter((x) => !x.archived_at && x.kind !== 'retired').map((x) => ({ label: x.name, value: x.id })),
)
const statusLabel = (id: string) => statusChoices.value.find((o) => o.value === id)?.label ?? 'another status'
const canQuick = (a: AssetListItem) => canManage.value && !a.retired_at
const rows = computed(() => data.value?.items ?? [])
const quick = ref<AssetListItem | null>(null)
const quickDraft = reactive(emptyDraft())
const quickSaving = ref(false)
const { data: quickAsset } = useAsset(() => quick.value?.id)
const { data: quickType } = useAssetType(() => quickAsset.value?.asset_type.id)
// Lưu một ô thuộc tính sửa tại chỗ; gõ sai kiểu (chữ trong ô số) thì báo, không lưu
function saveAttr(a: AssetListItem, attr: { key: string; label: string; data_type: DataType }, text: string) {
  const err = attrInputError(attr.data_type, text)
  if (err) return notify.error(`${attr.label}: ${err}`)
  return actions.quickSave(a, {}, `${attr.label} changed`, { [attr.key]: formValueFrom(attr.data_type, text) })
}
// Ô thuộc tính: kiểu ô nhập theo kiểu dữ liệu; có/không và lựa chọn dùng danh sách thả xuống
type AttrDef = { data_type: string; options: { id: string; label: string; removed?: boolean }[] }
const attrKind = (x: AttrDef) => (x.data_type === 'date' ? 'date' : x.data_type === 'select' || x.data_type === 'boolean' ? 'select' : 'text')
const attrOptions = (x: AttrDef) =>
  x.data_type === 'boolean'
    ? [{ label: 'Yes', value: 'true' }, { label: 'No', value: 'false' }]
    : x.data_type === 'select'
      ? x.options.filter((o) => !o.removed).map((o) => ({ label: o.label, value: o.id }))
      : undefined
// khoá trường thuộc tính: "attr:<key>" để không trùng tên trường của tài sản
const attrKey = (k: string) => `attr:${k}`
const quickFields = computed<FieldDef[]>(() => {
  const base: FieldDef[] = [
    { key: 'name', label: 'Name', maxlength: 200 },
    { key: 'tag', label: 'Tag', lock: 'Tags never change.' },
    { key: 'status_id', label: 'Status', kind: 'select', options: statusChoices.value },
    { key: 'purchase_date', label: 'Purchase date', kind: 'date' },
    { key: 'description', label: 'Description', kind: 'textarea' },
  ]
  const attrs = (quickType.value?.attributes ?? []).filter((x) => !x.removed).sort((x, y) => x.position - y.position)
  return base.concat(
    attrs.map((x) => {
      const label = x.unit ? `${x.label} (${x.unit})` : x.label
      if (x.data_type === 'date') return { key: attrKey(x.key), label, kind: 'date' }
      if (x.data_type === 'boolean')
        return { key: attrKey(x.key), label, kind: 'select', options: [{ label: '—', value: '' }, { label: 'Yes', value: 'true' }, { label: 'No', value: 'false' }] }
      if (x.data_type === 'select')
        return {
          key: attrKey(x.key),
          label,
          kind: 'select',
          options: [{ label: '—', value: '' }, ...x.options.filter((o) => !o.removed).map((o) => ({ label: o.label, value: o.id }))],
        }
      return { key: attrKey(x.key), label }
    }),
  )
})
// giá trị form của thuộc tính → chuỗi để so với bản nháp
const asText = (v: FormValues[string]) => (v instanceof Date ? (toDateString(v) ?? '') : v === null || v === undefined ? '' : String(v))
const quickSaved = computed(() => {
  const a = quickAsset.value
  if (!a) return {}
  const values = fromApiValues(a.attributes, a.attributes)
  return {
    name: a.name,
    tag: a.tag,
    status_id: a.status.id,
    purchase_date: a.purchase_date ?? '',
    description: a.description,
    ...Object.fromEntries(a.attributes.map((x) => [attrKey(x.key), asText(values[x.key])])),
  }
})
// mở/đóng ngăn kéo: mục giữ lại đến khi trượt ra xong rồi mới xoá cùng bản nháp
const quickDrawer = useQuickDrawer(quick, () => clearTab(quickDraft, 'overview'))
const quickOpen = quickDrawer.visible
function openQuick(a: AssetListItem) {
  quickDrawer.open(a)
}
const quickIndex = computed(() => (quick.value ? rows.value.findIndex((x) => x.id === quick.value!.id) : -1))
async function moveQuick(step: number) {
  const next = rows.value[quickIndex.value + step]
  if (!next || !(await mayClose(isDirty(quickDraft)))) return
  openQuick(next)
}
async function saveQuick() {
  const row = quick.value
  const a = quickAsset.value
  if (!row || !a) return
  const ch = changesOf(quickDraft, 'overview')
  const change: Record<string, unknown> = {}
  for (const k of ['name', 'status_id', 'description'] as const) if (k in ch) change[k] = String(ch[k])
  if ('purchase_date' in ch) change.purchase_date = ch.purchase_date ? String(ch.purchase_date) : undefined
  // chỉ gửi thuộc tính vừa sửa; quickSave đọc bản mới nhất và giữ nguyên các thuộc tính khác
  const attrPatch: FormValues = {}
  for (const k of Object.keys(ch).filter((x) => x.startsWith('attr:'))) {
    const key = k.slice(5)
    const def = a.attributes.find((x) => x.key === key)
    if (!def) continue
    // gõ sai kiểu thì không lưu gì, bản nháp giữ nguyên để sửa lại
    const err = attrInputError(def.data_type, String(ch[k] ?? ''))
    if (err) return notify.error(`${def.label}: ${err}`)
    attrPatch[key] = formValueFrom(def.data_type, String(ch[k] ?? ''))
  }
  quickSaving.value = true
  try {
    if (await actions.quickSave(row, change, 'saved', attrPatch)) clearTab(quickDraft, 'overview')
  } finally {
    quickSaving.value = false
  }
}
watch(rows, (list) => {
  if (quick.value) quick.value = list.find((x) => x.id === quick.value!.id) ?? null
})
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
        <Button v-if="canManage" as="router-link" :to="newPath" style="text-decoration: none" :aria-label="`${newLabel} (N)`">
          <i class="pi pi-plus" aria-hidden="true" />
          <span>{{ newLabel }}</span>
          <KeyHint keys="N" />
        </Button>
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
      <Button v-if="canManage" label="Change status" icon="pi pi-tag" size="small" @click="openBulk('status')" />
      <Button v-if="canManage" label="Retire" icon="pi pi-ban" size="small" severity="secondary" outlined @click="openBulk('retire')" />
      <Button
        v-if="canExport"
        label="Export selected"
        icon="pi pi-download"
        size="small"
        outlined
        :loading="exporting"
        :disabled="exporting"
        @click="exportSelected"
      />
      <Button v-if="canExport" label="Report from selected…" icon="pi pi-file-edit" size="small" outlined :disabled="exporting" @click="openReport(true)" />
      <Button label="Clear selection" size="small" text severity="secondary" @click="selected = []" />
    </div>
    <BulkActionDialog v-model:visible="bulkOpen" :mode="bulkMode" :rows="selected" @done="selected = []" />
    <ReportDialog v-if="canExport" v-model:visible="reportOpen" :scope="exportScope" :profile-id="reportProfile" />
    <DataExportDialog v-if="insideScope" v-model:visible="insideOpen" :scope="insideScope" />

    <RowMenus :menu="rowMenu" />
    <DataTable
      ref="tableRef"
      :value="data?.items ?? []"
      lazy
      :paginator="!isLoading"
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
      v-bind="table.bind"
      @page="onPage"
      @sort="onSort"
    >
      <Column v-if="canManage || canExport" selection-mode="multiple" header-style="width: 3rem" body-class="select-cell" />
      <Column header="Tag" sort-field="tag" sortable body-class="tag-cell">
        <template #body="{ data: a }: { data: AssetListItem }">
          <RouterLink :to="`/assets/${a.id}`">{{ a.tag }}</RouterLink>
        </template>
      </Column>
      <Column header="Name" sort-field="name" sortable>
        <template #body="{ data: a }: { data: AssetListItem }">
          <InlineCell :value="a.name" label="name" :editable="canQuick(a)" @save="(v) => actions.quickSave(a, { name: v }, `renamed to ${v}`)">
            {{ a.name }}
          </InlineCell>
        </template>
      </Column>
      <Column field="asset_type_name" header="Type" sort-field="asset_type" sortable />
      <Column header="Status" sort-field="status" sortable>
        <template #body="{ data: a }: { data: AssetListItem }">
          <InlineCell
            :value="a.status_id"
            label="status"
            :editable="canQuick(a)"
            kind="select"
            :options="statusChoices"
            @save="(v) => actions.quickSave(a, { status_id: v }, `set to ${statusLabel(v)}`)"
           
          >
            <Tag :value="a.status_name" :severity="kindSeverity(a.status_kind)" />
          </InlineCell>
        </template>
      </Column>
      <Column
        v-for="attr in attributes"
        :key="attr.id"
        :header="attr.unit ? `${attr.label} (${attr.unit})` : attr.label"
        :sort-field="`attributes.${attr.key}`"
        sortable
      >
        <template #body="{ data: a }: { data: AssetListItem }">
          <InlineCell
            :value="attrText(a.attributes?.find((x) => x.key === attr.key))"
            :label="attr.label"
            :editable="canQuick(a)"
            :kind="attrKind(attr)"
            :options="attrOptions(attr)"
            @save="(v) => saveAttr(a, attr, v)"
          >
            {{ cell(a, attr.key) }}
          </InlineCell>
        </template>
      </Column>
      <Column header="Purchased" sort-field="purchase_date" sortable>
        <template #body="{ data: a }: { data: AssetListItem }">
          <InlineCell
            :value="a.purchase_date ?? ''"
            label="purchase date"
            :editable="canQuick(a)"
            kind="date"
            @save="(v) => actions.quickSave(a, { purchase_date: v }, 'purchase date changed')"
           
          >
            {{ formatDate(a.purchase_date) || '—' }}
          </InlineCell>
        </template>
      </Column>
      <Column header="Updated" sort-field="updated_at" sortable>
        <template #body="{ data: a }: { data: AssetListItem }">{{ formatDateTime(a.updated_at) }}</template>
      </Column>
      <!-- nút ☰ luôn ở mép phải của bảng, kể cả khi bảng cuộn ngang -->
      <Column frozen align-frozen="right" header-class="row-menu-col" body-class="row-menu-col">
        <template #body="{ data: a, index }: { data: AssetListItem; index: number }">
          <RowMenuButton :active="table.active.isActive(index)" @open="(e) => rowMenu.toggle(a, e)" />
        </template>
      </Column>
      <template #empty>
        <TableSkeleton v-if="isLoading" :rows="12" />
        <EmptyState v-else icon="pi pi-box" text="No assets match these filters." />
      </template>
    </DataTable>

    <QuickEditDrawer
      v-if="quick"
      v-model:visible="quickOpen"
      @closed="quickDrawer.closed()"
      :title="`${quick.tag} · ${quick.name}`"
      icon="box"
      :dirty="isDirty(quickDraft)"
      :busy="quickSaving"
      :can-prev="quickIndex > 0"
      :can-next="quickIndex >= 0 && quickIndex < rows.length - 1"
      @save="saveQuick"
      @prev="moveQuick(-1)"
      @next="moveQuick(1)"
      @open-page="router.push(`/assets/${quick.id}`)"
    >
      <OverviewFields v-if="quickAsset && quickAsset.id === quick.id" :fields="quickFields" :saved="quickSaved" :draft="quickDraft" stacked />
      <TableSkeleton v-else />
    </QuickEditDrawer>

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
