<script setup lang="ts">
import Checkbox from 'primevue/checkbox'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Select from 'primevue/select'
import Tab from 'primevue/tab'
import TabList from 'primevue/tablist'
import TabPanel from 'primevue/tabpanel'
import TabPanels from 'primevue/tabpanels'
import Tabs from 'primevue/tabs'
import { computed, ref, watch } from 'vue'
import SegmentedFilter from '@/components/SegmentedFilter.vue'
import type { AssetListItem, ExportLayout } from '@/lib/api/types'
import { useSession } from '@/lib/auth/session'
import {
  cleanSheetName,
  cloneLayout,
  defaultReportLayout,
  editorColumns,
  excludeFields,
  fieldOptions,
  includeAttributes,
  previewSheets,
  sheetNameInput,
  skippedKeys,
  withColumns,
  type EditorColumn,
  type TypeInfo,
} from '../layout'
import ColumnEditor from './ColumnEditor.vue'
import SheetPreview from './SheetPreview.vue'

// Trình sửa bố cục báo cáo: cột, bố cục, định dạng và bản xem trước. Dùng chung cho hộp
// thoại "Export report" và trang profile. Nạp lại từ source mỗi khi sourceKey đổi; báo bố
// cục đang sửa qua update:current. hideTabs: trang profile tự có tab (Columns, Layout, Format)
const props = defineProps<{
  source: ExportLayout
  sourceKey: string | number
  types: TypeInfo[]
  rows: AssetListItem[]
  scopeLabel: string
  rowCount: number
  fileName: string
  titleName?: string
  hideTabs?: boolean
}>()
const emit = defineEmits<{ 'update:current': [layout: ExportLayout] }>()
// Tab cấu hình đang mở (cột, bố cục, định dạng); cha giữ để nhớ khi đóng mở lại
const tab = defineModel<'columns' | 'layout' | 'format'>('tab', { default: 'columns' })

const session = useSession()
const types = computed(() => props.types)
const rows = computed(() => props.rows)
const options = computed(() => fieldOptions(types.value))
const layout = ref<ExportLayout>(defaultReportLayout())
const columns = ref<EditorColumn[]>([])

// load: nạp bố cục (của profile hay mặc định) và dựng lại cột của trình sửa
function load() {
  layout.value = cloneLayout(props.source)
  autoTicked.value = []
  columns.value = editorColumns(layout.value, options.value)
}
// khi biết thêm loại: giữ cột đang sửa, chỉ thêm trường mới (chưa chọn) và bỏ trường không còn
watch(options, (opts) => {
  const keep = columns.value.filter((c) => c.include || opts.some((o) => o.field === c.field))
  const missing = opts.filter((o) => !keep.some((c) => c.field === o.field)).map((o) => ({ field: o.field, header: '', width: 0, include: false }))
  columns.value = [...keep, ...missing]
  // đang bật "Add each type's own attributes": thuộc tính của loại vừa nạp cũng được tick
  if (layout.value.sheets === 'per_type' && layout.value.each_type_attrs && autoTicked.value.length) {
    const r = includeAttributes(columns.value, opts)
    columns.value = r.columns
    autoTicked.value = [...autoTicked.value, ...r.added]
  }
})

// tên sheet được làm sạch lần cuối (bỏ dấu nháy đơn ở đầu/cuối) trước khi lưu hay tải
const current = computed(() => ({ ...withColumns(layout.value, columns.value), sheet_name: cleanSheetName(layout.value.sheet_name) }))
// ô nhập tên sheet: thay ký tự Excel cấm bằng '-' ngay khi gõ
const sheetName = computed({
  get: () => layout.value.sheet_name,
  set: (v) => (layout.value = { ...layout.value, sheet_name: sheetNameInput(v ?? '') }),
})
const sheets = computed(() => previewSheets(current.value, rows.value, types.value))
// sheet đang xem ở bản xem trước; sheet của một loại thì trình sửa cột chỉ hiện cột của loại đó
const activeSheet = ref(0)
const scope = computed<TypeInfo[] | null>(() => {
  const s = sheets.value[activeSheet.value]
  return s?.typeId ? types.value.filter((t) => t.id === s.typeId) : null
})
const scopeName = computed(() => (scope.value ? sheets.value[activeSheet.value].name : ''))
const skipped = computed(() => skippedKeys(current.value, types.value))
const title = computed(() =>
  current.value.title_row ? [props.titleName ?? 'Asset report', `Generated ${new Date().toLocaleDateString('en-GB')} by ${session.me?.account.name ?? ''} · ${props.scopeLabel}`] : [],
)

// Luôn hiện ở tab Columns; chỉ có nghĩa khi mỗi loại một sheet, nên tick lúc đang một
// sheet thì chuyển sang mỗi loại một sheet
// Tick: các cột thuộc tính cũng được tick trong danh sách (thấy trước cái sẽ có trong file);
// bỏ tick: chỉ bỏ những cột chính nó đã tick
const autoTicked = ref<string[]>([])
const eachTypeAttrs = computed({
  get: () => layout.value.sheets === 'per_type' && !!layout.value.each_type_attrs,
  set: (on: boolean) => {
    layout.value = { ...layout.value, each_type_attrs: on, sheets: on ? 'per_type' : layout.value.sheets }
    if (on) {
      const r = includeAttributes(columns.value, options.value)
      columns.value = r.columns
      autoTicked.value = r.added
    } else {
      columns.value = excludeFields(columns.value, autoTicked.value)
      autoTicked.value = []
    }
  },
})
// Có cột nào để xuất không (mỗi loại một sheet có thể chỉ dùng thuộc tính riêng của loại)
const hasColumns = computed(() => current.value.columns.length > 0 || (current.value.sheets === 'per_type' && !!current.value.each_type_attrs))
// Dòng dưới bản xem trước: số sheet, số dòng, tên file như server đặt
const sheetCount = computed(() => sheets.value.length + (current.value.summary ? 1 : 0))

const sheetsOptions = [
  { label: 'One sheet', value: 'single' as const },
  { label: 'Sheet per type', value: 'per_type' as const },
]
const sheetMode = computed({ get: () => layout.value.sheets, set: (v) => (layout.value = { ...layout.value, sheets: v }) })

const headerOptions = [
  { label: 'Plain', value: 'plain' },
  { label: 'Bold', value: 'bold' },
  { label: 'Bold with fill', value: 'bold_fill' },
]
const dateOptions = [
  { label: '06/10/2026', value: 'dd/mm/yyyy' },
  { label: '2026-10-06', value: 'yyyy-mm-dd' },
  { label: '6 Oct 2026', value: 'd mmm yyyy' },
]
const unitOptions = [
  { label: 'In the header', value: 'header' },
  { label: 'In each cell', value: 'cell' },
]
const boolOptions = [
  { label: 'Yes / No', value: 'yes_no' },
  { label: '✓ / –', value: 'check' },
]
const statusOptions = [
  { label: 'Status name', value: 'name' },
  { label: 'Kind', value: 'kind' },
]
// '' (theo sort của danh sách) đổi thành 'list' vì Select coi chuỗi rỗng là chưa chọn
const sortOptions = [
  { label: "List's sort", value: 'list' },
  { label: 'Tag', value: 'tag' },
  { label: 'Name', value: 'name' },
  { label: 'Purchase date', value: 'purchase_date' },
  { label: 'Type', value: 'asset_type' },
  { label: 'Status', value: 'status' },
]
const sortModel = computed({ get: () => layout.value.sort || 'list', set: (v) => (layout.value.sort = v === 'list' ? '' : v) })

watch(() => props.sourceKey, load, { immediate: true })
watch(current, (c) => emit('update:current', c), { immediate: true })
defineExpose({ current, hasColumns })
</script>

<template>
  <div class="body">
    <Tabs v-model:value="tab" class="config" :class="{ 'no-tabs': hideTabs }">
      <TabList v-if="!hideTabs">
        <Tab value="columns">Columns <span class="count">{{ current.columns.length }}</span></Tab>
        <Tab value="layout">Layout</Tab>
        <Tab value="format">Format</Tab>
      </TabList>
      <TabPanels>
        <TabPanel value="columns">
          <div class="each-type">
            <span class="check">
              <Checkbox v-model="eachTypeAttrs" input-id="each-type" binary />
              <label for="each-type">Add each type's own attributes</label>
            </span>
            <small v-if="layout.sheets === 'single'" class="hint">Ticking this switches to one sheet per type.</small>
          </div>
          <p v-if="scope" class="hint">Showing the columns of the {{ scopeName }} sheet. Changes apply to every sheet that has them.</p>
          <ColumnEditor v-model="columns" :options="options" :layout="layout" :scope="scope" />
          <span class="hint">Drag or press Alt+↑/↓ to reorder. Leave a header empty to use the default.</span>
        </TabPanel>
        <TabPanel value="layout">
          <SegmentedFilter v-model="sheetMode" :options="sheetsOptions" label="Sheets" />
          <div v-if="layout.sheets === 'single'" class="field">
            <label for="sheet-name">Sheet name</label>
            <InputText id="sheet-name" v-model="sheetName" maxlength="31" fluid />
          </div>
          <div class="checks">
            <span class="check"><Checkbox v-model="layout.title_row" input-id="title-row" binary /><label for="title-row">Title row</label></span>
            <span class="check"><Checkbox v-model="layout.summary" input-id="summary" binary /><label for="summary">Summary sheet</label></span>
          </div>
        </TabPanel>
        <TabPanel value="format">
          <div class="grid">
            <div class="field"><label for="f-header">Header style</label><Select v-model="layout.header" input-id="f-header" :options="headerOptions" option-label="label" option-value="value" fluid /></div>
            <div class="field"><label for="f-date">Dates</label><Select v-model="layout.date_format" input-id="f-date" :options="dateOptions" option-label="label" option-value="value" fluid /></div>
            <div class="field"><label for="f-unit">Units</label><Select v-model="layout.unit_in" input-id="f-unit" :options="unitOptions" option-label="label" option-value="value" fluid /></div>
            <div class="field"><label for="f-bool">Yes/no fields</label><Select v-model="layout.bool_style" input-id="f-bool" :options="boolOptions" option-label="label" option-value="value" fluid /></div>
            <div class="field"><label for="f-status">Status shows</label><Select v-model="layout.status_as" input-id="f-status" :options="statusOptions" option-label="label" option-value="value" fluid /></div>
            <div class="field"><label for="f-sort">Sort</label><Select v-model="sortModel" input-id="f-sort" :options="sortOptions" option-label="label" option-value="value" fluid /></div>
          </div>
          <div class="checks">
            <span class="check"><Checkbox v-model="layout.freeze" input-id="f-freeze" binary /><label for="f-freeze">Freeze header</label></span>
            <span class="check"><Checkbox v-model="layout.filter" input-id="f-filter" binary /><label for="f-filter">Filter buttons</label></span>
            <span class="check"><Checkbox v-model="layout.stripes" input-id="f-stripes" binary /><label for="f-stripes">Striped rows</label></span>
          </div>
        </TabPanel>
      </TabPanels>
    </Tabs>
    <div class="preview">
      <h3>Preview <small class="muted">first 20 rows of each sheet</small></h3>
      <Message v-if="skipped.length" severity="warn" :closable="false">
        Not available for the asset types in this export: <b>{{ skipped.join(', ') }}</b>. Those columns are skipped.
      </Message>
      <SheetPreview v-if="hasColumns" v-model:active="activeSheet" class="fill" :sheets="sheets" :layout="current" :types="types" :title="title" />
      <p v-else class="fill empty">Tick at least one column.</p>
      <div class="preview-meta">
        <span>{{ sheetCount }} {{ sheetCount === 1 ? 'sheet' : 'sheets' }}</span>
        <span>{{ rowCount }} {{ rowCount === 1 ? 'row' : 'rows' }}</span>
        <span>File: {{ fileName }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.body {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(0, 23rem) minmax(0, 1fr);
  grid-template-rows: minmax(0, 1fr);
}
.config {
  display: flex;
  flex-direction: column;
  min-height: 0;
  border-right: 1px solid var(--app-line);
}
.config :deep(.p-tablist) {
  flex: none;
}
.config :deep(.p-tabpanels) {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  scrollbar-gutter: stable;
  padding: 0.9rem 1.1rem 1.2rem;
}
.config :deep(.p-tabpanel) {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.count {
  margin-left: 0.4rem;
  padding: 0.05rem 0.45rem;
  border-radius: 999px;
  background: var(--app-soft);
  color: var(--p-text-muted-color);
  font: 0.72rem var(--app-mono);
}
.each-type {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}
.hint {
  font-size: 0.82rem;
  color: var(--p-text-muted-color);
}
.preview h3 {
  margin-bottom: 0.1rem;
}
.preview {
  overflow: hidden;
  padding: 0.9rem 1.1rem;
  background: var(--app-ground);
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  min-width: 0;
  min-height: 0;
}
.preview > * {
  flex: none;
}
.preview > .fill {
  flex: 1 1 auto;
  min-height: 0;
}
.empty {
  display: grid;
  place-items: center;
  border: 1px dashed var(--app-line);
  border-radius: 8px;
  color: var(--p-text-muted-color);
}
.preview-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem 1rem;
  font-size: 0.82rem;
  color: var(--p-text-muted-color);
}
.field {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  margin-top: 0.6rem;
  min-width: 0;
}
.grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 0.75rem;
}
.checks {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem 1rem;
  margin-top: 0.6rem;
}
.check {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
}
.muted {
  color: var(--p-text-muted-color);
}
/* không có thanh tab riêng (trang profile): khung cấu hình cao hết cột */
.config.no-tabs :deep(.p-tabpanels) {
  padding-top: 1rem;
}
/* Màn hẹp: xếp chồng, cả vùng cuộn như cũ */
@media (max-width: 900px) {
  .body {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: auto;
  }
  .config {
    border-right: 0;
    border-bottom: 1px solid var(--app-line);
  }
  .config :deep(.p-tabpanels),
  .preview {
    overflow: visible;
  }
  .preview :deep(.sheet-scroll) {
    max-height: 26rem;
  }
}
</style>
