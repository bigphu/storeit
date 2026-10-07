<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Popover from 'primevue/popover'
import Select from 'primevue/select'
import Tab from 'primevue/tab'
import TabList from 'primevue/tablist'
import TabPanel from 'primevue/tabpanel'
import TabPanels from 'primevue/tabpanels'
import Tabs from 'primevue/tabs'
import Tag from 'primevue/tag'
import { computed, ref, watch } from 'vue'
import FormDialog from '@/components/FormDialog.vue'
import SegmentedFilter from '@/components/SegmentedFilter.vue'
import { announce } from '@/lib/actions'
import type { ExportLayout, ExportProfile } from '@/lib/api/types'
import { useSession } from '@/lib/auth/session'
import { useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { useCreateExportProfile, useExportProfiles, useUpdateExportProfile } from '../api'
import {
  cleanSheetName,
  cloneLayout,
  defaultReportLayout,
  editorColumns,
  excludeFields,
  exportFileName,
  fieldOptions,
  includeAttributes,
  normalizeLayout,
  profilePatch,
  previewSheets,
  sheetNameInput,
  skippedKeys,
  withColumns,
  type EditorColumn,
} from '../layout'
import { useExport } from '../useExport'
import { type ExportScope, usePreviewData } from '../usePreviewData'
import ColumnEditor from './ColumnEditor.vue'
import SheetPreview from './SheetPreview.vue'

// Hộp thoại "Export report": chọn profile, chỉnh cột và định dạng, xem trước, tải về
const props = defineProps<{ scope: ExportScope; profileId?: string }>()
const visible = defineModel<boolean>('visible', { required: true })

const session = useSession()
const { data: profiles } = useExportProfiles(true)
const { types, rows } = usePreviewData(
  () => props.scope,
  () => visible.value,
)
const options = computed(() => fieldOptions(types.value))

const profileId = ref<string | null>(null)
const profile = computed<ExportProfile | undefined>(() => profiles.value?.find((p) => p.id === profileId.value))
const layout = ref<ExportLayout>(defaultReportLayout())
const columns = ref<EditorColumn[]>([])
// bố cục đã lưu của profile đang mở (null khi chưa chọn profile)
const saved = ref<string | null>(null)
// version của profile lúc nạp vào trình sửa: Save gửi version này (không phải bản mới nhất
// của danh sách profile) nên sửa đè lên thay đổi của người khác vẫn bị 409
const loadedVersion = ref<number | null>(null)

// load: nạp bố cục của profile (hay bố cục mặc định) và dựng lại cột của trình sửa.
// Chọn profile mà danh sách chưa về thì chờ, watcher bên dưới gọi lại khi có.
function load() {
  if (!visible.value) return
  const p = profile.value
  if (profileId.value && !p) return
  layout.value = p
    ? cloneLayout(p.layout)
    : { ...defaultReportLayout(), sheet_name: cleanSheetName(props.scope.label.slice(0, 31)) || 'Assets' }
  saved.value = p ? normalizeLayout(p.layout) : null
  loadedVersion.value = p?.version ?? null
  autoTicked.value = []
  columns.value = editorColumns(layout.value, options.value)
}
watch(visible, (open) => {
  if (!open) return
  profileId.value = props.profileId ?? null
  load()
})
watch([profileId, () => profile.value?.id], load)
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
const dirty = computed(() => saved.value !== null && normalizeLayout(current.value) !== saved.value)
const sheets = computed(() => previewSheets(current.value, rows.value, types.value))
const skipped = computed(() => skippedKeys(current.value, types.value))
const title = computed(() =>
  current.value.title_row ? [profile.value?.name ?? 'Asset report', `Generated ${new Date().toLocaleDateString('en-GB')} by ${session.me?.account.name ?? ''} · ${props.scope.label}`] : [],
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
const fileName = computed(() => exportFileName('report', profile.value?.name, new Date().toISOString().slice(0, 10)))

// Tab cấu hình đang mở (cột, bố cục, định dạng); giữ nguyên khi đóng mở lại hộp thoại
const tab = ref<'columns' | 'layout' | 'format'>('columns')

const sheetsOptions = [
  { label: 'One sheet', value: 'single' as const },
  { label: 'Sheet per type', value: 'per_type' as const },
]
const sheetMode = computed({ get: () => layout.value.sheets, set: (v) => (layout.value = { ...layout.value, sheets: v }) })

// Save…: một popover cho cả lưu vào profile này (tên, chia sẻ, cột và định dạng trong một
// PATCH) và lưu bản mới. Profile không sửa được, hay chưa chọn profile: chỉ Save as new.
const update = useUpdateExportProfile(false)
const create = useCreateExportProfile()
const errors = useFormErrors()
const saveForm = ref<{ name: string; shared: boolean; hint: string } | null>(null)
const savePop = ref<InstanceType<typeof Popover>>()
const canSaveHere = computed(() => !!profile.value?.can_edit)
// thay đổi Save sẽ gửi; null thì nút Save tắt
const patch = computed(() => {
  const p = profile.value
  if (!p || !saveForm.value) return null
  return profilePatch(p, saveForm.value, dirty.value ? current.value : null)
})
function openSave(e: Event) {
  errors.clear()
  const p = profile.value
  saveForm.value = p?.can_edit ? { name: p.name, shared: p.shared, hint: '' } : { name: p ? `${p.name} (copy)` : 'New report', shared: false, hint: '' }
  savePop.value?.toggle(e)
}
function closeSave() {
  savePop.value?.hide()
  saveForm.value = null
}
async function saveHere() {
  const p = profile.value
  const ch = patch.value
  if (!p || !ch || loadedVersion.value === null) return
  errors.clear()
  try {
    // bản trước lần lưu này, để Undo đặt lại
    const before = { name: p.name, shared: p.shared, layout: p.layout }
    const next = await update.mutateAsync({ id: p.id, version: loadedVersion.value, ...ch })
    saved.value = normalizeLayout(next.layout)
    loadedVersion.value = next.version
    closeSave()
    announce(next, {
      done: ch.name ? `Saved ${next.name} (renamed).` : `Saved ${next.name}.`,
      undo: async (n) => {
        const back = await update.mutateAsync({ id: p.id, version: n.version, name: before.name, shared: before.shared, layout: before.layout })
        // hộp thoại còn mở trên profile này: lấy bản vừa đặt lại
        if (profileId.value === p.id) {
          saved.value = normalizeLayout(back.layout)
          loadedVersion.value = back.version
        }
      },
      undone: `${before.name} put back as it was.`,
      undoFailed: `Couldn't put ${before.name} back. The saved version stays.`,
    })
  } catch (err) {
    errors.set(err)
  }
}
async function saveNew() {
  const f = saveForm.value
  if (!f) return
  const p = profile.value
  // tên chưa đổi trên profile của mình: gợi ý "(copy)" thay vì báo trùng tên
  if (p && p.owner.id === session.me?.account.id && f.name.trim() === p.name) {
    f.name = `${p.name} (copy)`
    f.hint = 'Pick a name for the copy, then Save as new again.'
    return
  }
  errors.clear()
  try {
    const np = await create.mutateAsync({ name: f.name, shared: f.shared, layout: current.value })
    closeSave()
    profileId.value = np.id
    notify.success(`Saved ${np.name}.`)
  } catch (err) {
    errors.set(err)
  }
}
// Enter trong ô tên: Save khi lưu được vào profile này, không thì Save as new
function submitSave() {
  if (canSaveHere.value && patch.value) saveHere()
  else saveNew()
}

const { run, running } = useExport()
async function download() {
  // lỗi (vd quá số dòng) thì giữ hộp thoại để không mất phần đã sửa
  const ok = await run({ mode: 'report', filters: props.scope.filters, layout: current.value, profile_id: profileId.value ?? undefined }, 'storeit-report.xlsx', {
    rows: props.scope.count,
  })
  if (ok) visible.value = false
}

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
</script>

<template>
  <FormDialog v-model:visible="visible" icon="file" title="Export report" width="max(80rem, 88vw)" flush :dirty="dirty" class="report-dialog">
    <!-- Cạnh tiêu đề: profile, trạng thái, lưu; không thêm hàng nào trên vùng cuộn -->
    <template #header-extra>
      <div class="title-bar">
        <Select v-model="profileId" aria-label="Export profile" :options="profiles ?? []" option-label="name" option-value="id" placeholder="No profile" show-clear size="small" class="profile-select" />
        <i v-if="profile && !profile.can_edit" v-tooltip.bottom="`Shared by ${profile.owner.name}`" class="pi pi-lock state" aria-label="Shared by someone else" />
        <i v-else-if="profile?.shared" v-tooltip.bottom="'Shared with everyone who can export'" class="pi pi-users state" aria-label="Shared" />
        <Tag v-if="dirty" value="Unsaved" severity="warn" class="unsaved" />
        <Button label="Save…" size="small" text aria-haspopup="dialog" @click="openSave" />
      </div>
    </template>
    <Popover ref="savePop" @hide="saveForm = null">
      <form v-if="saveForm" class="save-as" @submit.prevent="submitSave">
        <label for="save-name">Name</label>
        <InputText id="save-name" v-model="saveForm.name" required maxlength="100" autofocus fluid :invalid="!!errors.fields.value.name" @update:model-value="saveForm.hint = ''" />
        <span class="check">
          <Checkbox v-model="saveForm.shared" input-id="save-shared" binary />
          <label for="save-shared">Share with everyone who can export</label>
        </span>
        <Message v-if="errors.general.value || errors.fields.value.name" severity="error" size="small" variant="simple">{{ errors.fields.value.name ?? errors.general.value }}</Message>
        <small v-if="saveForm.hint" class="save-hint">{{ saveForm.hint }}</small>
        <div class="save-as-actions">
          <Button label="Cancel" size="small" text severity="secondary" @click="closeSave" />
          <Button label="Save as new" size="small" :severity="canSaveHere ? 'secondary' : undefined" :outlined="canSaveHere" :loading="create.isPending.value" @click="saveNew" />
          <Button v-if="canSaveHere" label="Save" size="small" :disabled="!patch" :loading="update.isPending.value" @click="saveHere" />
        </div>
      </form>
    </Popover>

    <div class="report">
      <div class="body">
        <Tabs v-model:value="tab" class="config">
          <TabList>
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
              <ColumnEditor v-model="columns" :options="options" :layout="layout" />
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
          <SheetPreview v-if="hasColumns" class="fill" :sheets="sheets" :layout="current" :types="types" :title="title" />
          <p v-else class="fill empty">Tick at least one column.</p>
          <div class="preview-meta">
            <span>{{ sheetCount }} {{ sheetCount === 1 ? 'sheet' : 'sheets' }}</span>
            <span>{{ scope.count }} {{ scope.count === 1 ? 'row' : 'rows' }}</span>
            <span>File: {{ fileName }}</span>
          </div>
        </div>
      </div>
    </div>
    <template #footer="{ close }">
      <span v-tooltip.top="'Built on the server with the same filters as the list. Up to 50,000 rows.'" class="foot-note">
        <i class="pi pi-table" aria-hidden="true" />
        <span><b>{{ scope.count }}</b> {{ scope.count === 1 ? 'asset' : 'assets' }} · {{ scope.label }}</span>
        <span class="muted">· {{ scope.selection ? 'only the selected assets' : 'uses the list’s current filters' }}</span>
      </span>
      <!-- qua FormDialog: còn thay đổi chưa lưu thì hỏi như ✕ và Esc -->
      <Button label="Cancel" text severity="secondary" @click="close" />
      <Button label="Download .xlsx" icon="pi pi-download" :loading="running" :disabled="!current.columns.length && !(current.sheets === 'per_type' && current.each_type_attrs)" @click="download" />
    </template>
  </FormDialog>
</template>

<style scoped>
.title-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem 0.6rem;
  flex: 1;
  min-width: 0;
}
.state {
  color: var(--p-text-muted-color);
}
.unsaved {
  font-size: 0.72rem;
}
.save-as {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  width: min(20rem, 80vw);
}
.save-lead {
  font-size: 0.84rem;
  color: var(--p-text-muted-color);
}
.save-hint {
  color: var(--p-primary-color);
}
.save-as-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.4rem;
}
.profile-select {
  min-width: 13rem;
}
/* Chiều cao cố định: thanh profile và dòng phạm vi đứng yên, cột cấu hình và
   bản xem trước mỗi bên tự cuộn */
.report {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
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
/* minmax(0, 1fr): cột không nở theo nhãn dài của Select (vd "Purchase date") */
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
.foot-note {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.45rem;
  margin-right: auto;
  text-align: left;
  font-size: 0.88rem;
  line-height: 1.2;
}
/* Màn hẹp: xếp chồng, cả hộp thoại cuộn như cũ */
@media (max-width: 900px) {
  .report {
    height: auto;
  }
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

<style>
/* Hộp thoại cao cố định, nội dung không cuộn: chỉ cột cấu hình và bản xem trước cuộn.
   Dialog teleport ra body nên khối này không scoped. Màn hẹp: cả hộp thoại cuộn như cũ. */
.p-dialog.report-dialog {
  height: min(50rem, 94vh);
}
.p-dialog.report-dialog .p-dialog-content {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}
@media (max-width: 900px) {
  .p-dialog.report-dialog {
    height: auto;
  }
  .p-dialog.report-dialog .p-dialog-content {
    overflow: auto;
  }
}
</style>
