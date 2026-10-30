<script setup lang="ts">
import Textarea from 'primevue/textarea'
import { computed, ref, watch } from 'vue'
import FormDialog from '@/components/FormDialog.vue'
import type { BorrowTicket } from '@/lib/api/types'
import { useDirty, useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { useRejectTicket } from '../api'

// Từ chối phiếu đang chờ: lý do bắt buộc, người gửi nhận thư kèm lý do
const props = defineProps<{ ticket: BorrowTicket | null }>()
const visible = defineModel<boolean>('visible', { required: true })

const reject = useRejectTicket()
const errors = useFormErrors()
const reason = ref('')
const form = useDirty(() => reason.value.trim())
watch(
  visible,
  (open) => {
    if (!open) return
    reason.value = ''
    errors.clear()
    form.reset()
  },
  { immediate: true },
)
const problem = computed(() => (!reason.value.trim() ? 'Give a reason.' : reason.value.trim().length > 500 ? 'Keep the reason under 500 characters.' : null))

async function submit() {
  const t = props.ticket
  if (!t || problem.value) return
  errors.clear()
  try {
    await reject.mutateAsync({ id: t.id, reason: reason.value.trim(), version: t.version })
    notify.success(`Ticket #${t.number} rejected.`)
    visible.value = false
  } catch (err) {
    errors.set(err)
  }
}
</script>

<template>
  <FormDialog
    v-model:visible="visible"
    icon="error"
    :title="ticket ? `Reject ticket #${ticket.number}` : 'Reject ticket'"
    action="Reject ticket"
    :busy="reject.isPending.value"
    :error="errors.general.value"
    :dirty="form.dirty.value"
    :disabled="!!problem"
    @submit="submit"
  >
    <p v-if="ticket" class="note">{{ ticket.requester.name }} gets an email with this reason.</p>
    <div class="field">
      <label for="reject-reason">Reason</label>
      <Textarea id="reject-reason" v-model="reason" rows="3" maxlength="500" auto-resize fluid />
      <small v-if="errors.fields.value.reason" class="field-error">{{ errors.fields.value.reason }}</small>
    </div>
  </FormDialog>
</template>

<style scoped>
.note {
  margin: 0 0 0.75rem;
  color: var(--p-text-muted-color);
}
</style>
