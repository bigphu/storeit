<script setup lang="ts">
import Checkbox from 'primevue/checkbox'
import Column from 'primevue/column'
import DataTable, { type DataTableRowReorderEvent } from 'primevue/datatable'
import InputText from 'primevue/inputtext'
import type { ExportLayout } from '@/lib/api/types'
import { defaultHeader, type EditorColumn, type FieldOption } from '../layout'

// Chọn, sắp (kéo hay Alt+↑/↓) và đổi tên cột của báo cáo
const props = defineProps<{ options: FieldOption[]; layout: ExportLayout }>()
const columns = defineModel<EditorColumn[]>({ required: true })

const option = (f: string) => props.options.find((o) => o.field === f)
function onReorder(e: DataTableRowReorderEvent) {
  columns.value = e.value as EditorColumn[]
}
function move(i: number, d: number) {
  const j = i + d
  if (j < 0 || j >= columns.value.length) return
  const next = [...columns.value]
  ;[next[i], next[j]] = [next[j], next[i]]
  columns.value = next
}
function set(i: number, patch: Partial<EditorColumn>) {
  columns.value = columns.value.map((c, n) => (n === i ? { ...c, ...patch } : c))
}
</script>

<template>
  <DataTable :value="columns" data-key="field" :show-headers="false" size="small" class="col-editor" table-style="width: 100%; table-layout: fixed" @row-reorder="onReorder">
    <Column row-reorder header-style="width: 2rem" body-style="width: 2rem" />
    <Column header-style="width: 2rem" body-style="width: 2rem">
      <template #body="{ data: c, index: i }: { data: EditorColumn; index: number }">
        <Checkbox :model-value="c.include" binary :aria-label="`Include ${option(c.field)?.label ?? c.field}`" @update:model-value="(v: boolean) => set(i, { include: v })" />
      </template>
    </Column>
    <Column>
      <template #body="{ data: c, index: i }: { data: EditorColumn; index: number }">
        <div class="label" :class="{ off: !c.include }" tabindex="0" @keydown.alt.up.prevent="move(i, -1)" @keydown.alt.down.prevent="move(i, 1)">
          <span>{{ option(c.field)?.label ?? c.field }}</span>
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
.label.off {
  opacity: 0.5;
}
</style>
