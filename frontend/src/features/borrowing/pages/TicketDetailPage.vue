<script setup lang="ts">
import Button from 'primevue/button'
import Chip from 'primevue/chip'
import Panel from 'primevue/panel'
import Tag from 'primevue/tag'
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import AppBreadcrumb from '@/components/AppBreadcrumb.vue'
import DetailHeader from '@/components/DetailHeader.vue'
import EmptyState from '@/components/EmptyState.vue'
import { useTabTitle } from '@/app/tabs/tabPage'
import { formatDate, formatDateTime } from '@/lib/dates'
import { useAssetType } from '@/features/asset-types/api'
import { useTicket } from '../api'
import { requestCode, requirementLabels, TICKET_STATE_LABEL, TICKET_STATE_TONE, ticketLoanLabel } from '../borrowing'
import ApproveDialog from '../components/ApproveDialog.vue'
import RejectDialog from '../components/RejectDialog.vue'
import { useTicketActions } from '../useTicketActions'

// Trang một phiếu mượn: thông tin, yêu cầu, tài sản được giữ chỗ, và hành động theo trạng thái
// và quyền (duyệt, từ chối, giao, huỷ). Người khác mở phiếu không phải của mình thì API trả 404.
const props = defineProps<{ id: string }>()
const { data: ticket, isLoading } = useTicket(() => props.id)
const { data: type } = useAssetType(() => ticket.value?.asset_type.id)
const specs = computed(() => (ticket.value ? requirementLabels(ticket.value.requirements, type.value?.attributes ?? []) : []))
const actions = useTicketActions()

useTabTitle(() => (ticket.value ? requestCode(ticket.value.number) : undefined))
const crumbs = computed(() => [
  actions.canManage.value ? { label: 'Requests', to: '/borrowing/tickets' } : { label: 'My requests', to: '/borrowing/mine' },
  { label: ticket.value ? requestCode(ticket.value.number) : '…' },
])
</script>

<template>
  <section v-if="ticket">
    <AppBreadcrumb :items="crumbs" />
    <DetailHeader :title="requestCode(ticket.number)" icon="box">
      <template #tags>
        <Tag :value="TICKET_STATE_LABEL[ticket.state]" :severity="TICKET_STATE_TONE[ticket.state]" />
      </template>
      <div>
        {{ ticket.requester.name }} asked for a {{ ticket.asset_type.name }} · sent {{ formatDateTime(ticket.created_at) }}
      </div>
      <template #actions>
        <Button v-if="actions.can.approve(ticket)" label="Approve…" icon="pi pi-check" @click="actions.askApprove(ticket)" />
        <Button
          v-if="actions.can.handOver(ticket)"
          label="Hand over"
          icon="pi pi-arrow-right-arrow-left"
          @click="actions.doHandOver(ticket)"
        />
        <Button v-if="actions.can.reject(ticket)" label="Reject…" severity="secondary" outlined @click="actions.askReject(ticket)" />
        <Button v-if="actions.can.cancel(ticket)" label="Cancel request" severity="secondary" outlined @click="actions.doCancel(ticket)" />
      </template>
    </DetailHeader>

    <div class="panels">
      <Panel header="Request">
        <dl class="facts">
          <dt>Requested by</dt>
          <dd>{{ ticket.requester.name }} <span class="muted">· {{ ticket.requester.email }}</span></dd>
          <dt>Type</dt>
          <dd>{{ ticket.asset_type.name }}</dd>
          <dt>Specs</dt>
          <dd>
            <span v-if="!specs.length" class="muted">None</span>
            <span v-else class="chips"><Chip v-for="(s, i) in specs" :key="i" :label="s" /></span>
          </dd>
          <dt>Needed from</dt>
          <dd>{{ formatDate(ticket.needed_from) }}</dd>
          <dt>Return by</dt>
          <dd>{{ ticket.return_by ? formatDate(ticket.return_by) : 'Not set' }}</dd>
          <template v-if="ticket.suggested_asset">
            <dt>Asked for</dt>
            <dd>
              <RouterLink :to="`/assets/${ticket.suggested_asset.id}`" class="mono">{{ ticket.suggested_asset.tag }}</RouterLink>
              · {{ ticket.suggested_asset.name }}
            </dd>
          </template>
          <template v-if="ticket.note">
            <dt>Note</dt>
            <dd class="pre">{{ ticket.note }}</dd>
          </template>
        </dl>
      </Panel>
      <Panel header="Decision">
        <dl class="facts">
          <dt>Status</dt>
          <dd>{{ TICKET_STATE_LABEL[ticket.state] }}</dd>
          <template v-if="ticket.asset">
            <dt>Asset</dt>
            <dd>
              <RouterLink :to="`/assets/${ticket.asset.id}`" class="mono">{{ ticket.asset.tag }}</RouterLink> · {{ ticket.asset.name }}
            </dd>
          </template>
          <template v-if="ticket.loan">
            <dt>Loan</dt>
            <dd>{{ ticketLoanLabel(ticket.loan) }}</dd>
          </template>
          <template v-if="ticket.decided_at">
            <dt>Decided</dt>
            <dd>{{ formatDateTime(ticket.decided_at) }}</dd>
          </template>
          <template v-if="ticket.cancelled_at">
            <dt>Cancelled</dt>
            <dd>{{ formatDateTime(ticket.cancelled_at) }}</dd>
          </template>
          <template v-if="ticket.reason">
            <dt>Reason</dt>
            <dd class="pre">{{ ticket.reason }}</dd>
          </template>
        </dl>
        <p v-if="ticket.state === 'waiting'" class="muted">Waiting for the inventory team.</p>
        <p v-else-if="ticket.state === 'approved'" class="muted">Reserved and ready for pickup from the inventory team.</p>
      </Panel>
    </div>
    <ApproveDialog v-model:visible="actions.approveOpen.value" :ticket="actions.approveTarget.value" />
    <RejectDialog v-model:visible="actions.rejectOpen.value" :ticket="actions.rejectTarget.value" :cancel="actions.rejectCancel.value" />
  </section>
  <EmptyState v-else-if="!isLoading" icon="pi pi-ticket" text="This request doesn't exist, or it isn't yours." />
</template>

<style scoped>
.panels {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr));
  gap: 1rem;
  margin-top: 1rem;
}
.facts {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 0.5rem 1rem;
  margin: 0;
}
.facts dt {
  color: var(--p-text-muted-color);
}
.facts dd {
  margin: 0;
}
.chips {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}
.muted {
  color: var(--p-text-muted-color);
}
.mono {
  font-family: var(--app-font-mono, ui-monospace, monospace);
}
.pre {
  white-space: pre-wrap;
}
</style>
