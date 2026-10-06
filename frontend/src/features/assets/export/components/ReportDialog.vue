<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Select from 'primevue/select'
import Tag from 'primevue/tag'
import { computed, ref, watch } from 'vue'
import SegmentedFilter from '@/components/SegmentedFilter.vue'
import type { ExportLayout, ExportProfile } from '@/lib/api/types'
import { useSession } from '@/lib/auth/session'
import { useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { useCreateExportProfile, useExportProfiles, useUpdateExportProfile } from '../api'
import { defaultReportLayout, editorColumns, fieldOptions, normalizeLayout, previewSheets, skippedKeys, withColumns, type EditorColumn } from '../layout'
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

// load: nạp bố cục của profile (hay bố cục mặc định) và dựng lại cột của trình sửa.
// Chọn profile mà danh sách chưa về thì chờ, watcher bên dưới gọi lại khi có.
function load() {
  if (!visible.value) return
  const p = profile.value
  if (profileId.value && !p) return
  layout.value = p
    ? structuredClone(p.layout)
    : { ...defaultReportLayout(), sheet_name: props.scope.label.slice(0, 31).replace(/[[\]:*?/\\]/g, '-') || 'Assets' }
  saved.value = p ? normalizeLayout(p.layout) : null
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
})

const current = computed(() => withColumns(layout.value, columns.value))
const dirty = computed(() => saved.value !== null && normalizeLayout(current.value) !== saved.value)
const sheets = computed(() => previewSheets(current.value, rows.value, types.value))
const skipped = computed(() => skippedKeys(current.value, types.value))
const title = computed(() =>
  current.value.title_row ? [profile.value?.name ?? 'Asset report', `Generated ${new Date().toLocaleDateString('en-GB')} by ${session.me?.account.name ?? ''} · ${props.scope.label}`] : [],
)

const sheetsOptions = [
  { label: 'One sheet', value: 'single' as const },
  { label: 'Sheet per type', value: 'per_type' as const },
]
const sheetMode = computed({ get: () => layout.value.sheets, set: (v) => (layout.value = { ...layout.value, sheets: v }) })

// Lưu: sửa profile của mình (hay có quyền quản lý); "Save as…" tạo bản mới
const update = useUpdateExportProfile()
const create = useCreateExportProfile()
const errors = useFormErrors()
const saveAs = ref<{ name: string; shared: boolean } | null>(null)
async function save() {
  const p = profile.value
  if (!p) return
  const next = await update.mutateAsync({ id: p.id, version: p.version, layout: current.value })
  saved.value = normalizeLayout(next.layout)
  notify.success(`Saved ${p.name}.`)
}
function openSaveAs() {
  errors.clear()
  saveAs.value = { name: profile.value ? `${profile.value.name} (copy)` : 'New report', shared: false }
}
async function submitSaveAs() {
  if (!saveAs.value) return
  errors.clear()
  try {
    const p = await create.mutateAsync({ name: saveAs.value.name, shared: saveAs.value.shared, layout: current.value })
    saveAs.value = null
    profileId.value = p.id
    notify.success(`Saved ${p.name}.`)
  } catch (err) {
    errors.set(err)
  }
}

const { run, running } = useExport()
async function download() {
  // lỗi (vd quá số dòng) thì giữ hộp thoại để không mất phần đã sửa
  const ok = await run({ mode: 'report', filters: props.scope.filters, layout: current.value, profile_id: profileId.value ?? undefined }, 'storeit-report.xlsx')
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
  <Dialog v-model:visible="visible" modal header="Export report" :style="{ width: 'min(72rem, 96vw)' }" :content-style="{ padding: 0 }">
    <div class="head">
      <label for="report-profile" class="muted">Profile</label>
      <Select v-model="profileId" input-id="report-profile" :options="profiles ?? []" option-label="name" option-value="id" placeholder="No profile" show-clear class="profile-select" />
      <Tag v-if="profile && !profile.can_edit" :value="`Shared by ${profile.owner.name}`" icon="pi pi-lock" severity="secondary" />
      <Tag v-else-if="profile" :value="profile.shared ? 'Shared' : 'Only you'" severity="secondary" />
      <Tag v-if="dirty" value="Unsaved changes" severity="warn" />
      <Button label="Save" size="small" severity="secondary" outlined :disabled="!profile?.can_edit || !dirty" :loading="update.isPending.value" @click="save" />
      <Button label="Save as…" size="small" severity="secondary" outlined @click="openSaveAs" />
    </div>
    <form v-if="saveAs" class="sub" @submit.prevent="submitSaveAs">
      <label for="save-as-name">Name</label>
      <InputText id="save-as-name" v-model="saveAs.name" required maxlength="100" autofocus :invalid="!!errors.fields.value.name" />
      <span class="check">
        <Checkbox v-model="saveAs.shared" input-id="save-as-shared" binary />
        <label for="save-as-shared">Share with everyone who can export</label>
      </span>
      <Button type="submit" label="Save profile" size="small" :loading="create.isPending.value" />
      <Button label="Cancel" size="small" text severity="secondary" @click="saveAs = null" />
      <Message v-if="errors.general.value || errors.fields.value.name" severity="error" size="small" variant="simple">{{ errors.fields.value.name ?? errors.general.value }}</Message>
    </form>
    <div v-else class="sub">
      <i class="pi pi-table" /> Exporting <b>{{ scope.count }}</b> assets · {{ scope.label }}
    </div>

    <div class="body">
      <div class="config">
        <section>
          <h3>Columns</h3>
          <ColumnEditor v-model="columns" :options="options" :layout="layout" />
          <div v-if="layout.sheets === 'per_type'" class="check">
            <Checkbox v-model="layout.each_type_attrs" input-id="each-type" binary />
            <label for="each-type">Add each type's own attributes</label>
          </div>
        </section>
        <section>
          <h3>Layout</h3>
          <SegmentedFilter v-model="sheetMode" :options="sheetsOptions" label="Sheets" />
          <div v-if="layout.sheets === 'single'" class="field">
            <label for="sheet-name">Sheet name</label>
            <InputText id="sheet-name" v-model="layout.sheet_name" maxlength="31" fluid />
          </div>
          <div class="checks">
            <span class="check"><Checkbox v-model="layout.title_row" input-id="title-row" binary /><label for="title-row">Title row</label></span>
            <span class="check"><Checkbox v-model="layout.summary" input-id="summary" binary /><label for="summary">Summary sheet</label></span>
          </div>
        </section>
        <section>
          <h3>Formatting</h3>
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
        </section>
      </div>
      <div class="preview">
        <h3>Preview <small class="muted">first 20 rows of each sheet</small></h3>
        <Message v-if="skipped.length" severity="warn" :closable="false">
          This layout names attributes that no longer exist: <b>{{ skipped.join(', ') }}</b>. Those columns are skipped.
        </Message>
        <SheetPreview :sheets="sheets" :layout="current" :types="types" :title="title" />
      </div>
    </div>
    <template #footer>
      <span class="muted foot-note">Built on the server with the same filters as the list. Up to 50,000 rows.</span>
      <Button label="Cancel" text severity="secondary" @click="visible = false" />
      <Button label="Download .xlsx" icon="pi pi-download" :loading="running" :disabled="!current.columns.length && !(current.sheets === 'per_type' && current.each_type_attrs)" @click="download" />
    </template>
  </Dialog>
</template>

<style scoped>
.head,
.sub {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.6rem;
  padding: 0.7rem 1.1rem;
  border-bottom: 1px solid var(--app-line);
}
.sub {
  background: var(--app-ground);
  font-size: 0.88rem;
}
.profile-select {
  min-width: 13rem;
}
.body {
  display: grid;
  grid-template-columns: minmax(0, 23rem) minmax(0, 1fr);
  max-height: 70vh;
}
.config {
  overflow-y: auto;
  padding: 0.9rem 1.1rem;
  border-right: 1px solid var(--app-line);
  display: flex;
  flex-direction: column;
  gap: 1.1rem;
}
.config h3,
.preview h3 {
  margin-bottom: 0.5rem;
}
.preview {
  overflow: auto;
  padding: 0.9rem 1.1rem;
  background: var(--app-ground);
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  min-width: 0;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  margin-top: 0.6rem;
}
.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
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
  margin-right: auto;
  font-size: 0.85rem;
}
@media (max-width: 900px) {
  .body {
    grid-template-columns: minmax(0, 1fr);
    max-height: none;
  }
  .config {
    border-right: 0;
    border-bottom: 1px solid var(--app-line);
  }
}
</style>
