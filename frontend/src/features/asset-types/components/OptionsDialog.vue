<script setup lang="ts">
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable, { type DataTableRowReorderEvent } from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import { useConfirm } from 'primevue/useconfirm'
import { reactive, ref, watch } from 'vue'
import type { Attribute, Option } from '@/lib/api/types'
import { notify } from '@/lib/notify'
import { useAddOption, useRemoveOption, useReorderOptions, useUpdateOption } from '../api'

// Option của một thuộc tính select: kéo để đổi thứ tự (thứ tự trong form và khi sắp
// theo cột), sửa nhãn, thêm, gỡ. attribute lấy từ query chi tiết loại nên tự cập nhật
const props = defineProps<{ typeId: string; attribute: Attribute | null; canManage: boolean }>()
const visible = defineModel<boolean>('visible', { required: true })

const confirm = useConfirm()
const add = useAddOption()
const update = useUpdateOption()
const remove = useRemoveOption()
const reorder = useReorderOptions()

// thứ tự đang hiện: đổi ngay khi thả, không đợi tải lại
const rows = ref<Option[]>([])
const labels = reactive<Record<string, string>>({})
watch(
  () => props.attribute?.options,
  (opts) => {
    rows.value = (opts ?? []).filter((o) => !o.removed).sort((a, b) => a.position - b.position)
    for (const o of rows.value) labels[o.id] = o.label
  },
  { immediate: true },
)

const newLabel = ref('')
const ids = () => ({ typeId: props.typeId, attrId: props.attribute!.id })

function onReorder(e: DataTableRowReorderEvent) {
  rows.value = e.value as Option[]
  reorder.mutate({ ...ids(), ids: rows.value.map((o) => o.id) })
}

function save(o: Option) {
  update
    .mutateAsync({ ...ids(), optionId: o.id, label: labels[o.id] })
    .then(() => notify.success('Option saved.'))
    .catch(() => {})
}

function addOption() {
  if (!newLabel.value.trim()) return
  // option mới đứng cuối
  const position = Math.max(0, ...rows.value.map((o) => o.position)) + 1
  add
    .mutateAsync({ ...ids(), label: newLabel.value.trim(), position })
    .then(() => (newLabel.value = ''))
    .catch(() => {})
}

function askRemove(o: Option) {
  confirm.require({
    message: `Remove the option ${o.label}? Assets that have it keep it, shown as removed.`,
    header: 'Confirm',
    acceptLabel: 'Remove',
    rejectLabel: 'Cancel',
    accept: () =>
      remove
        .mutateAsync({ ...ids(), optionId: o.id })
        .then(() => notify.success('Option removed.'))
        .catch(() => {}),
  })
}
</script>

<template>
  <Dialog v-model:visible="visible" modal :header="`Options of ${attribute?.label ?? ''}`" :style="{ width: 'min(92vw, 34rem)' }">
    <p v-if="canManage" class="hint">Drag the handle to change the order.</p>
    <DataTable :value="rows" data-key="id" size="small" @row-reorder="onReorder">
      <Column v-if="canManage" row-reorder header-style="width: 2.5rem" />
      <Column header="Label">
        <template #body="{ data: o }: { data: Option }">
          <InputText v-if="canManage" v-model="labels[o.id]" aria-label="Label" fluid @keydown.enter="save(o)" />
          <span v-else>{{ o.label }}</span>
        </template>
      </Column>
      <Column v-if="canManage" header="" body-class="actions-cell">
        <template #body="{ data: o }: { data: Option }">
          <Button
            v-tooltip.top="'Save'"
            icon="pi pi-check"
            size="small"
            text
            rounded
            aria-label="Save"
            :disabled="labels[o.id] === o.label"
            @click="save(o)"
          />
          <Button
            v-tooltip.top="'Remove'"
            icon="pi pi-trash"
            size="small"
            text
            rounded
            severity="danger"
            aria-label="Remove"
            @click="askRemove(o)"
          />
        </template>
      </Column>
      <template #empty>No options yet.</template>
    </DataTable>
    <form v-if="canManage" class="actions add" @submit.prevent="addOption">
      <InputText v-model="newLabel" placeholder="New option" aria-label="New option" />
      <Button type="submit" label="Add" :loading="add.isPending.value" />
    </form>
  </Dialog>
</template>

<style scoped>
.hint {
  margin: 0 0 0.5rem;
  color: var(--p-text-muted-color);
  font-size: 0.875rem;
}
.add {
  margin-top: 1rem;
}
:deep(.actions-cell) {
  white-space: nowrap;
  text-align: right;
}
</style>
