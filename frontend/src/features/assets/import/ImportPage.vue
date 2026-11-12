<script setup lang="ts">
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Message from 'primevue/message'
import Tag from 'primevue/tag'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import EmptyState from '@/components/EmptyState.vue'
import PageHeader from '@/components/PageHeader.vue'
import SegmentedFilter, { type SegmentOption } from '@/components/SegmentedFilter.vue'
import type { ImportReport } from '@/lib/api/types'
import { saveBlob } from '@/lib/download'
import { describeError } from '@/lib/errors'
import { notify } from '@/lib/notify'
import { downloadImportTemplate, useImportCommit, useImportPreview } from './api'
import { filterRows, importable, problemsCsv, resultLabel, resultTone, summaryLine, type ImportFilter } from './format'

// Import tài sản từ Excel (US-06): Upload -> Review (xem trước, chưa ghi gì) -> Done.
// Giữ file trong bộ nhớ; "Import" gửi lại đúng file đó, server lập kế hoạch lại rồi lưu từng dòng.
const router = useRouter()
const preview = useImportPreview()
const commit = useImportCommit()

const file = ref<File | null>(null)
const report = ref<ImportReport | null>(null)
const fileError = ref('')
const dragging = ref(false)
const filter = ref<ImportFilter>('all')
const input = ref<HTMLInputElement | null>(null)

const step = computed<'upload' | 'review' | 'done'>(() => (!report.value ? 'upload' : report.value.committed ? 'done' : 'review'))
const busy = computed(() => preview.isPending.value || commit.isPending.value)

const filterOptions = computed<SegmentOption<ImportFilter>[]>(() => {
  const c = report.value?.counts
  return [
    { label: 'All', value: 'all', count: report.value?.rows.length ?? 0 },
    { label: 'Create', value: 'create', count: c?.create ?? 0 },
    { label: 'Update', value: 'update', count: c?.update ?? 0 },
    { label: 'No change', value: 'skip', count: c?.skip ?? 0 },
    { label: 'Failed', value: 'fail', count: c?.fail ?? 0 },
  ]
})
const rows = computed(() => filterRows(report.value?.rows ?? [], filter.value))

async function pick(f: File | undefined) {
  if (!f || busy.value) return
  fileError.value = ''
  if (!f.name.toLowerCase().endsWith('.xlsx')) {
    fileError.value = 'Choose an Excel workbook (.xlsx).'
    return
  }
  try {
    report.value = await preview.mutateAsync(f)
    file.value = f
    filter.value = report.value.counts.fail ? 'fail' : 'all'
  } catch (err) {
    fileError.value = describeError(err)
  }
}
function onDrop(e: DragEvent) {
  dragging.value = false
  void pick(e.dataTransfer?.files[0])
}
function onInput(e: Event) {
  const el = e.target as HTMLInputElement
  void pick(el.files?.[0])
  el.value = '' // chọn lại cùng file vẫn kích hoạt change
}

async function confirm() {
  if (!file.value || busy.value) return
  try {
    report.value = await commit.mutateAsync(file.value)
    filter.value = 'all'
  } catch (err) {
    // vd file đã được import: quay lại bước tải lên với lý do
    const message = describeError(err)
    reset()
    fileError.value = message
  }
}
function reset() {
  file.value = null
  report.value = null
  filter.value = 'all'
  fileError.value = ''
}
function saveProblems() {
  if (!report.value) return
  const base = report.value.file_name.replace(/\.xlsx$/i, '') || 'import'
  saveBlob(new Blob([problemsCsv(report.value.rows)], { type: 'text/csv;charset=utf-8' }), `${base}-problems.csv`)
}
async function template() {
  try {
    notify.success(`Downloaded ${await downloadImportTemplate()}.`)
  } catch (err) {
    notify.error(describeError(err))
  }
}
</script>

<template>
  <div class="import-page">
    <PageHeader title="Import assets" subtitle="Create new assets and update existing ones from an Excel workbook.">
      <Button label="Download template" icon="pi pi-download" size="small" outlined severity="secondary" @click="template" />
    </PageHeader>

    <section v-if="step === 'upload'" class="panel">
      <div :class="['drop', { dragging }]" @dragover.prevent="dragging = true" @dragleave="dragging = false" @drop.prevent="onDrop">
        <i :class="busy ? 'pi pi-spin pi-spinner' : 'pi pi-file-excel'" aria-hidden="true" />
        <p>{{ busy ? 'Checking the file…' : 'Drop an .xlsx file here' }}</p>
        <Button label="Choose file" size="small" :disabled="busy" @click="input?.click()" />
        <input ref="input" type="file" accept=".xlsx" hidden @change="onInput" />
        <small>One sheet per asset type, named by the type code. Up to 5 MB and 5000 rows. Nothing is saved until you confirm.</small>
      </div>
      <Message v-if="fileError" severity="error" :closable="false">{{ fileError }}</Message>
    </section>

    <section v-else-if="report" class="panel">
      <div class="summary">
        <div>
          <strong>{{ report.file_name }}</strong>
          <p class="muted">{{ summaryLine(report) }}</p>
        </div>
        <div class="actions">
          <Button v-if="report.counts.fail" label="Problems (.csv)" icon="pi pi-download" size="small" outlined severity="secondary" @click="saveProblems" />
          <template v-if="step === 'review'">
            <Button label="Choose another file" size="small" text :disabled="busy" @click="reset" />
            <Button
              :label="`Import ${importable(report)} ${importable(report) === 1 ? 'row' : 'rows'}`"
              icon="pi pi-upload"
              size="small"
              :loading="commit.isPending.value"
              :disabled="!importable(report)"
              @click="confirm"
            />
          </template>
          <template v-else>
            <Button label="Import another file" size="small" outlined @click="reset" />
            <Button label="View assets" icon="pi pi-arrow-right" icon-pos="right" size="small" @click="router.push('/assets')" />
          </template>
        </div>
      </div>
      <Message v-if="step === 'review' && report.counts.fail" severity="warn" :closable="false">
        Failed rows are left out; the rest can still be imported. Fix the file and import it again for the failed rows.
      </Message>
      <Message v-else-if="step === 'done'" severity="success" :closable="false">Import finished. Failed rows, if any, were not saved.</Message>

      <SegmentedFilter v-model="filter" :options="filterOptions" label="Show rows" />
      <DataTable :value="rows" size="small" paginator :rows="50" :rows-per-page-options="[50, 100, 500]">
        <template #empty><EmptyState icon="pi pi-inbox" text="No rows in this view." /></template>
        <Column field="sheet" header="Sheet" />
        <Column field="row" header="Row" />
        <Column field="tag" header="Tag" />
        <Column field="name" header="Name" />
        <Column header="Result">
          <template #body="{ data }">
            <Tag :severity="resultTone(data.result)" :value="resultLabel(data.result, report.committed)" />
          </template>
        </Column>
        <Column field="message" header="Details" />
      </DataTable>
    </section>
  </div>
</template>

<style scoped>
.import-page {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.panel {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.drop {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  padding: 2.5rem 1rem;
  text-align: center;
  border: 2px dashed var(--app-line);
  border-radius: 0.75rem;
  color: var(--p-text-muted-color);
}
.drop.dragging {
  border-color: var(--app-accent);
}
.drop i {
  font-size: 2rem;
}
.summary {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 0.75rem;
}
.summary p {
  margin: 0.2rem 0 0;
}
.muted {
  color: var(--p-text-muted-color);
}
.actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}
</style>
