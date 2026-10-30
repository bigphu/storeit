<script setup lang="ts">
import Textarea from 'primevue/textarea'
import { computed, ref, watch } from 'vue'
import FormDialog from '@/components/FormDialog.vue'
import type { BorrowTicket } from '@/lib/api/types'
import { useDirty, useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { useCancelTicket, useRejectTicket } from '../api'

// Từ chối phiếu đang chờ (lý do bắt buộc), hoặc người quản lý huỷ phiếu đã duyệt của người khác
// (cancel: lý do tuỳ chọn). Người gửi nhận thư kèm lý do
const props = withDefaults(defineProps<{ ticket: BorrowTicket | null; cancel?: boolean }>(), { cancel: false })
const visible = defineModel<boolean>('visible', { required: true })

const reject = useRejectTicket()
const cancelTicket = useCancelTicket()
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
const problem = computed(() => {
  const r = reason.value.trim()
  if (!r && !props.cancel) return 'Give a reason.'
  return r.length > 500 ? 'Keep the reason under 500 characters.' : null
})
const busy = computed(() => reject.isPending.value || cancelTicket.isPending.value)
const verb = computed(() => (props.cancel ? 'Cancel' : 'Reject'))

async function submit() {
  const t = props.ticket
  if (!t || problem.value) return
  errors.clear()
  try {
    const body = { id: t.id, reason: reason.value.trim(), version: t.version }
    if (props.cancel) await cancelTicket.mutateAsync(body)
    else await reject.mutateAsync(body)
    notify.success(`Ticket #${t.number} ${props.cancel ? 'cancelled' : 'rejected'}.`)
    visible.value = false
  } catch (err) {
    errors.set(err)
  }
}
</script>

<template>
  <FormDialog
    v-model:visible="visible"
    :icon="cancel ? 'alert' : 'error'"
    :title="ticket ? `${verb} ticket #${ticket.number}` : `${verb} ticket`"
    :action="`${verb} ticket`"
    :busy="busy"
    :error="errors.general.value"
    :dirty="form.dirty.value"
    :disabled="!!problem"
    @submit="submit"
  >
    <p v-if="ticket" class="note">
      {{ ticket.requester.name }} gets an email {{ cancel ? 'saying the ticket was cancelled, with your reason if you give one' : 'with this reason' }}.
      <template v-if="cancel && ticket.asset"> {{ ticket.asset.tag }} is no longer reserved and becomes available.</template>
    </p>
    <div class="field">
      <label for="reject-reason">Reason<span v-if="cancel" class="optional"> (optional)</span></label>
      <!-- autofocus: Dialog đặt con trỏ vào ô lý do khi mở -->
      <Textarea id="reject-reason" v-model="reason" rows="3" maxlength="500" auto-resize fluid autofocus />
      <small v-if="errors.fields.value.reason" class="field-error">{{ errors.fields.value.reason }}</small>
    </div>
  </FormDialog>
</template>

<style scoped>
.note {
  margin: 0 0 0.75rem;
  color: var(--p-text-muted-color);
}
.optional {
  color: var(--p-text-muted-color);
  font-weight: 400;
}
</style>
