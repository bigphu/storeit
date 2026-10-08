<script setup lang="ts">
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable, { type DataTableRowReorderEvent } from 'primevue/datatable'
import InputText from 'primevue/inputtext'
import { reactive, ref, watch } from 'vue'
import type { Attribute, Option } from '@/lib/api/types'
import EmptyState from '@/components/EmptyState.vue'
import FormDialog from '@/components/FormDialog.vue'
import IconAction from '@/components/IconAction.vue'
import RowActions from '@/components/RowActions.vue'
import { runAction } from '@/lib/actions'
import { notify } from '@/lib/notify'
import { useAddOption, useRemoveOption, useReorderOptions, useRestoreOption, useUpdateOption } from '../api'

// Option của một thuộc tính select: kéo để đổi thứ tự (thứ tự trong form và khi sắp
// theo cột), sửa nhãn, thêm, gỡ. attribute lấy từ query chi tiết loại nên tự cập nhật
const props = defineProps<{ typeId: string; attribute: Attribute | null; canManage: boolean }>()
const visible = defineModel<boolean>('visible', { required: true })

const add = useAddOption()
const update = useUpdateOption()
const removeOpt = useRemoveOption()
const restoreOpt = useRestoreOption()
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
  const before = rows.value.map((o) => o.id)
  rows.value = e.value as Option[]
  const after = rows.value.map((o) => o.id)
  void runAction({
    run: () => reorder.mutateAsync({ ...ids(), ids: after }),
    done: 'Options reordered.',
    failed: "Couldn't save the new order.",
    undo: () => reorder.mutateAsync({ ...ids(), ids: before }),
    undone: 'Order put back.',
    undoFailed: "Couldn't put the order back.",
  })
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

function removeOption(o: Option) {
  const target = { ...ids(), optionId: o.id }
  return runAction({
    run: () => removeOpt.mutateAsync(target),
    done: `${o.label} removed.`,
    failed: `Couldn't remove ${o.label}.`,
    undo: () => restoreOpt.mutateAsync(target),
    undone: `${o.label} is back.`,
    undoFailed: `Couldn't bring ${o.label} back. It stays removed.`,
  })
}
</script>

<template>
  <FormDialog v-model:visible="visible" size="l" icon="sliders" :title="`Options of ${attribute?.label ?? ''}`">
    <p v-if="canManage" class="hint">Drag the handle to change the order.</p>
    <DataTable :value="rows" data-key="id" size="small" row-hover @row-reorder="onReorder">
      <Column v-if="canManage" row-reorder row-reorder-icon="pi pi-arrows-v" header-style="width: 2.5rem" />
      <Column header="Label">
        <template #body="{ data: o }: { data: Option }">
          <InputText v-if="canManage" v-model="labels[o.id]" aria-label="Label" fluid @keydown.enter="save(o)" />
          <span v-else>{{ o.label }}</span>
        </template>
      </Column>
      <Column v-if="canManage" header="" body-class="actions-cell">
        <!-- bảng sửa trong hộp thoại: nút luôn hiện (không đợi rê chuột) -->
        <template #body="{ data: o }: { data: Option }">
          <RowActions :count="2">
            <IconAction icon="pi pi-check" label="Save" :disabled="labels[o.id] === o.label" reason="No changes to save" @click="save(o)" />
            <IconAction icon="pi pi-trash" label="Remove" danger @click="removeOption(o)" />
          </RowActions>
        </template>
      </Column>
      <template #empty><EmptyState icon="pi pi-list" text="No options yet." /></template>
    </DataTable>
    <form v-if="canManage" class="actions add" @submit.prevent="addOption">
      <InputText v-model="newLabel" placeholder="New option" aria-label="New option" />
      <Button type="submit" label="Add" :loading="add.isPending.value" />
    </form>
  </FormDialog>
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
