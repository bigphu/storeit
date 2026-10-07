<script setup lang="ts">
import Textarea from 'primevue/textarea'
import { ref, watch } from 'vue'
import FormDialog from '@/components/FormDialog.vue'
import { announce } from '@/lib/actions'
import { useDirty, useFormErrors } from '@/lib/forms'
import { useRestoreAsset, useRetireAsset } from '../api'

// Hộp thoại retire dùng chung cho danh sách và trang tài sản; giữ ô lý do, báo kèm Undo
const props = defineProps<{ asset: { id: string; tag: string; name: string; version: number } | null }>()
const visible = defineModel<boolean>('visible', { required: true })
const emit = defineEmits<{ retired: [] }>()

const reason = ref('')
const retire = useRetireAsset()
const restore = useRestoreAsset()
const errors = useFormErrors()
const form = useDirty(() => reason.value.trim())

watch(visible, (open) => {
  if (!open) return
  reason.value = ''
  errors.clear()
  form.reset()
})

async function submit() {
  const a = props.asset
  if (!a) return
  errors.clear()
  try {
    const retired = await retire.mutateAsync({ id: a.id, reason: reason.value, version: a.version })
    visible.value = false
    emit('retired')
    announce(retired, {
      done: `${a.tag} retired.`,
      undo: (r) => restore.mutateAsync({ id: a.id, version: r.version }),
      undone: `${a.tag} restored.`,
      undoFailed: `Couldn't restore ${a.tag}. It is still retired.`,
    })
  } catch (err) {
    errors.set(err)
  }
}
</script>

<template>
  <FormDialog
    v-model:visible="visible"
    size="s"
    icon="box"
    :title="asset ? `Retire ${asset.tag}` : 'Retire asset'"
    action="Retire"
    :busy="retire.isPending.value"
    :error="errors.general.value"
    :dirty="form.dirty.value"
    @submit="submit"
  >
    <p>{{ asset?.name }} leaves the list unless “Include retired” is on, and can't be edited until restored.</p>
    <div class="field">
      <label for="retire-reason">Reason (optional)</label>
      <Textarea id="retire-reason" v-model="reason" rows="3" />
    </div>
  </FormDialog>
</template>
