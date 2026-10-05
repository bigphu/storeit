<script setup lang="ts">
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import { useConfirm } from 'primevue/useconfirm'
import { computed, reactive, ref, watch } from 'vue'
import type { Attribute } from '@/lib/api/types'
import { notify } from '@/lib/notify'
import { useAddOption, useRemoveOption, useUpdateOption } from '../api'

// Sửa option của một thuộc tính select; attribute lấy từ query chi tiết loại nên tự cập nhật
const props = defineProps<{ typeId: string; attribute: Attribute | null; canManage: boolean }>()
const visible = defineModel<boolean>('visible', { required: true })

const confirm = useConfirm()
const add = useAddOption()
const update = useUpdateOption()
const remove = useRemoveOption()

const active = computed(() => (props.attribute?.options ?? []).filter((o) => !o.removed))
// bản nháp nhãn và vị trí theo id option
const drafts = reactive<Record<string, { label: string; position: number }>>({})
watch(
  active,
  (opts) => {
    for (const o of opts) drafts[o.id] = { label: o.label, position: o.position }
  },
  { immediate: true },
)

const newLabel = ref('')

function ids() {
  return { typeId: props.typeId, attrId: props.attribute!.id }
}

async function save(optionId: string) {
  const d = drafts[optionId]
  await update
    .mutateAsync({ ...ids(), optionId, label: d.label, position: d.position })
    .then(() => notify.success('Option saved.'))
    .catch(() => {})
}

async function addOption() {
  if (!newLabel.value.trim()) return
  const position = Math.max(0, ...active.value.map((o) => o.position)) + 1
  await add
    .mutateAsync({ ...ids(), label: newLabel.value.trim(), position })
    .then(() => (newLabel.value = ''))
    .catch(() => {})
}

function askRemove(optionId: string, label: string) {
  confirm.require({
    message: `Remove the option ${label}? Assets that have it keep it, shown as removed.`,
    header: 'Confirm',
    acceptLabel: 'Remove',
    rejectLabel: 'Cancel',
    accept: () =>
      remove
        .mutateAsync({ ...ids(), optionId })
        .then(() => notify.success('Option removed.'))
        .catch(() => {}),
  })
}
</script>

<template>
  <Dialog v-model:visible="visible" modal :header="`Options of ${attribute?.label ?? ''}`" :style="{ width: '36rem' }">
    <table class="options">
      <thead>
        <tr>
          <th>Label</th>
          <th>Position</th>
          <th v-if="canManage"></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="o in active" :key="o.id">
          <td><InputText v-model="drafts[o.id].label" :disabled="!canManage" aria-label="Label" /></td>
          <td>
            <InputNumber
              v-model="drafts[o.id].position"
              :disabled="!canManage"
              :use-grouping="false"
              aria-label="Position"
              input-class="narrow"
            />
          </td>
          <td v-if="canManage" class="actions">
            <Button label="Save" size="small" text @click="save(o.id)" />
            <Button label="Remove" size="small" text severity="danger" @click="askRemove(o.id, o.label)" />
          </td>
        </tr>
      </tbody>
    </table>
    <form v-if="canManage" class="actions add" @submit.prevent="addOption">
      <InputText v-model="newLabel" placeholder="New option" aria-label="New option" />
      <Button type="submit" label="Add" :loading="add.isPending.value" />
    </form>
  </Dialog>
</template>

<style scoped>
.options {
  width: 100%;
  border-collapse: collapse;
}
.options th {
  text-align: left;
}
.options td {
  padding: 0.25rem;
}
.add {
  margin-top: 1rem;
}
:deep(.narrow) {
  width: 5rem;
}
</style>
