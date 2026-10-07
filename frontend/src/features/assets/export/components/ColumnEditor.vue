<script setup lang="ts">
import Checkbox from 'primevue/checkbox'
import Column from 'primevue/column'
import DataTable, { type DataTableRowReorderEvent } from 'primevue/datatable'
import InputText from 'primevue/inputtext'
import Tag from 'primevue/tag'
import { nextTick, ref } from 'vue'
import type { ExportLayout } from '@/lib/api/types'
import { defaultHeader, type EditorColumn, type FieldOption } from '../layout'

// Chọn, sắp (kéo hay Alt+↑/↓) và đổi tên cột của báo cáo
const props = defineProps<{ options: FieldOption[]; layout: ExportLayout }>()
const columns = defineModel<EditorColumn[]>({ required: true })

const option = (f: string) => props.options.find((o) => o.field === f)
function onReorder(e: DataTableRowReorderEvent) {
  columns.value = e.value as EditorColumn[]
}
// Dòng có key theo field: đổi chỗ thì DataTable dời phần tử DOM của dòng, và trình duyệt
// bỏ focus của phần tử bị dời. Focus lại dòng vừa chuyển để Alt+↑/↓ bấm tiếp được.
const root = ref<HTMLElement>()
function focusRow(field: string) {
  root.value?.querySelector<HTMLElement>(`[data-field="${CSS.escape(field)}"]`)?.focus()
}
function move(i: number, d: number) {
  const j = i + d
  if (j < 0 || j >= columns.value.length) return
  const field = columns.value[i].field
  const next = [...columns.value]
  ;[next[i], next[j]] = [next[j], next[i]]
  columns.value = next
  nextTick(() => focusRow(field))
}
function set(i: number, patch: Partial<EditorColumn>) {
  columns.value = columns.value.map((c, n) => (n === i ? { ...c, ...patch } : c))
}
</script>

<template>
  <div ref="root">
    <DataTable :value="columns" data-key="field" :show-headers="false" size="small" class="col-editor" table-style="width: 100%; table-layout: fixed" @row-reorder="onReorder">
      <Column row-reorder header-style="width: 2rem" body-style="width: 2rem" />
      <Column header-style="width: 2rem" body-style="width: 2rem">
        <template #body="{ data: c, index: i }: { data: EditorColumn; index: number }">
          <Checkbox :model-value="c.include" binary :aria-label="`Include ${option(c.field)?.label ?? c.field}`" @update:model-value="(v: boolean) => set(i, { include: v })" />
        </template>
      </Column>
      <Column>
        <template #body="{ data: c, index: i }: { data: EditorColumn; index: number }">
          <div
            class="label"
            :class="{ off: !c.include }"
            :data-field="c.field"
            tabindex="0"
            :aria-label="`${option(c.field)?.label ?? c.field}: Alt+Up or Alt+Down to move`"
            @keydown.alt.up.prevent="move(i, -1)"
            @keydown.alt.down.prevent="move(i, 1)"
          >
            <span>
              {{ option(c.field)?.label ?? c.field }}
              <Tag v-if="!option(c.field)" v-tooltip="'No asset type in this export has this attribute, so the column is skipped'" value="Not available" severity="warn" class="na" />
            </span>
            <small>{{ c.field }}<template v-if="option(c.field)?.types.length"> · {{ option(c.field)!.types.join(', ') }}</template></small>
          </div>
        </template>
      </Column>
      <Column header-style="width: 9rem" body-style="width: 9rem">
        <template #body="{ data: c, index: i }: { data: EditorColumn; index: number }">
          <InputText :model-value="c.header" size="small" fluid :placeholder="defaultHeader(c.field, layout, options)" :disabled="!c.include" :aria-label="`Header for ${option(c.field)?.label ?? c.field}`" maxlength="100" @update:model-value="(v) => set(i, { header: v ?? '' })" />
        </template>
      </Column>
    </DataTable>
  </div>
</template>

<style scoped>
.col-editor {
  border: 0;
}
.col-editor :deep(td) {
  border-bottom: 0;
}
.label {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.label span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.label small {
  font: 0.7rem var(--app-mono);
  color: var(--p-text-muted-color);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.na {
  margin-left: 0.35rem;
  padding: 0 0.35rem;
  font-size: 0.68rem;
}
/* Bấm vào dòng: dòng được chọn (nền xanh như dòng chọn ở bảng khác), Alt+↑/↓ chuyển nó */
.label {
  outline: none;
  cursor: pointer;
}
.col-editor :deep(tr:has(.label:focus)) > td {
  background: var(--p-highlight-background);
}
.label:focus-visible {
  outline: 2px solid var(--app-brand);
  outline-offset: 2px;
  border-radius: 4px;
}
.label.off {
  opacity: 0.5;
}
</style>
