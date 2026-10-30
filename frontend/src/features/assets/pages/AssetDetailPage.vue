<script setup lang="ts">
import { useQueryClient } from '@tanstack/vue-query'
import Button from 'primevue/button'
import Panel from 'primevue/panel'
import Tab from 'primevue/tab'
import TabList from 'primevue/tablist'
import TabPanel from 'primevue/tabpanel'
import TabPanels from 'primevue/tabpanels'
import Tabs from 'primevue/tabs'
import Tag from 'primevue/tag'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useTabId, useTabTitle } from '@/app/tabs/tabPage'
import AppBreadcrumb, { type Crumb } from '@/components/AppBreadcrumb.vue'
import KeyHint from '@/components/KeyHint.vue'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { formatDate, formatDateTime } from '@/lib/dates'
import { usePageKeys } from '@/lib/pageKeys'
import { kindSeverity } from '@/features/statuses/api'
import { useAssetType } from '@/features/asset-types/api'
import { useTicket } from '@/features/borrowing/api'
import { canSeeTicket, overdueLabel } from '@/features/borrowing/borrowing'
import LendDialog from '@/features/borrowing/components/LendDialog.vue'
import NewTicketDialog from '@/features/borrowing/components/NewTicketDialog.vue'
import RejectDialog from '@/features/borrowing/components/RejectDialog.vue'
import ReturnDialog from '@/features/borrowing/components/ReturnDialog.vue'
import { useTicketActions } from '@/features/borrowing/useTicketActions'
import { fetchAssetPage, useAsset } from '../api'
import AssetActivity from '../components/AssetActivity.vue'
import ReplaceDialog from '../components/ReplaceDialog.vue'
import RetireDialog from '../components/RetireDialog.vue'
import { position, stepFrom, useListContext } from '../listContext'
import { listLocation, toApiParams, typeListLocation } from '../listQuery'
import { useAssetActions } from '../useAssetActions'
import { formatValue } from '../values'

const props = defineProps<{ id: string }>()

const session = useSession()
const router = useRouter()
const qc = useQueryClient()
const listContext = useListContext()
const tabId = useTabId()
const actions = useAssetActions()
const canManage = computed(() => session.can(Perm.AssetManage))
const canManageTypes = computed(() => session.can(Perm.TypeManage))

const { data: asset } = useAsset(() => props.id)
// tab đang mở; Activity chỉ tải khi mở lần đầu
const tab = ref<'details' | 'activity'>('details')
// hộp "Replace" (US-13)
const replaceOpen = ref(false)
const retired = computed(() => !!asset.value?.retired_at)

// Mượn (US-10): đang giữ chỗ / đang cho mượn thì không retire, thay hay đổi status được;
// tài sản rảnh của loại cho mượn được thì cho mượn trực tiếp hay gửi phiếu "Borrow this…"
const borrowing = computed(() => asset.value?.borrowing ?? null)
const { data: assetType } = useAssetType(() => asset.value?.asset_type.id)
const borrowable = computed(() => !retired.value && asset.value?.status.kind === 'available' && !!assetType.value?.can_borrow)
const canLend = computed(() => borrowable.value && session.can(Perm.BorrowManage))
const canAsk = computed(() => borrowable.value && session.can(Perm.BorrowRequest))
const ticketActions = useTicketActions()
// phiếu chỉ người quản lý và người mượn mở được (người khác: API 404, không tải, không link)
const seeTicket = computed(() => canSeeTicket(borrowing.value, session.me?.account.id, session.can(Perm.BorrowManage)))
const { data: heldTicket } = useTicket(() =>
  borrowing.value?.state === 'reserved' && seeTicket.value ? borrowing.value.ticket?.id : undefined,
)
const lendOpen = ref(false)
const askOpen = ref(false)
// đang cho mượn: người quản lý ghi nhận trả (US-11)
const canReturn = computed(() => borrowing.value?.state === 'on_loan' && !!borrowing.value.since && session.can(Perm.BorrowManage))
const returnOpen = ref(false)
const returnTarget = computed(() =>
  asset.value && borrowing.value?.since ? { id: asset.value.id, tag: asset.value.tag, version: asset.value.version, startedOn: borrowing.value.since } : null,
)
useTabTitle(() => asset.value?.tag)

// Danh sách đã mở trước đó (nếu tài sản này nằm trong nó): quay lại và bước qua kết quả
const ctx = computed(() => {
  const c = listContext.ctxFor(tabId)
  return c && c.ids.includes(props.id) ? c : null
})
const pos = computed(() => (ctx.value ? position(ctx.value, props.id) : null))

const crumbs = computed<Crumb[]>(() => {
  const a = asset.value
  if (!a) return []
  const c = ctx.value
  const allTo = c && !c.state.typeId ? listLocation(c.state) : '/assets'
  const typeTo =
    c && c.state.typeId === a.asset_type.id ? listLocation(c.state) : typeListLocation(a.asset_type.id, listContext.views)
  return [{ label: 'Assets', to: allTo }, { label: a.asset_type.name, to: typeTo }, { label: a.tag }]
})

// Bước tới/lui: trong trang thì lấy id kề bên; qua trang thì tải trang đó (có cache)
async function step(dir: 1 | -1) {
  const c = ctx.value
  if (!c) return
  const s = stepFrom(c, props.id, dir)
  if (!s) return
  if ('id' in s) {
    await router.replace(`/assets/${s.id}`)
    return
  }
  const state = { ...c.state, page: s.page }
  const page = await fetchAssetPage(qc, toApiParams(state, c.pageSize))
  const ids = page.items.map((a) => a.id)
  const next = s.pick === 'first' ? ids[0] : ids[ids.length - 1]
  if (!next) return
  listContext.setCtx(tabId, { state, pageSize: c.pageSize, ids, total: page.total })
  await router.replace(`/assets/${next}`)
}

// Phím tắt (chỉ khi trang đang hiện): J/K bước qua danh sách, E sửa
usePageKeys((e) => {
  if (e.key === 'j') step(-1)
  else if (e.key === 'k') step(1)
  else if (e.key === 'e' && canManage.value && asset.value && !retired.value) actions.edit(asset.value)
})
</script>

<template>
  <section v-if="asset">
    <AppBreadcrumb :items="crumbs" />
    <div class="page-header">
      <h1 class="asset-title">
        <span class="tag-no">{{ asset.tag }}</span>
        <span>{{ asset.name }}</span>
        <Tag :value="asset.status.name" :severity="kindSeverity(asset.status.kind)" />
      </h1>
      <span v-if="pos" class="stepper" aria-label="Position in the list">
        <Button icon="pi pi-angle-left" text rounded aria-label="Previous asset (J)" title="Previous (J)" :disabled="pos.index === 0" @click="step(-1)" />
        <span>{{ pos.index + 1 }} of {{ pos.total }}</span>
        <Button icon="pi pi-angle-right" text rounded aria-label="Next asset (K)" title="Next (K)" :disabled="pos.index + 1 >= pos.total" @click="step(1)" />
      </span>
    </div>

    <div v-if="canManage || canLend || canAsk || canReturn || (heldTicket && ticketActions.canManage.value)" class="actions page-actions">
      <template v-if="canManage">
        <template v-if="!retired">
          <Button aria-label="Edit (E)" @click="(e: MouseEvent) => actions.edit(asset!, e)">
            <i class="pi pi-pencil" aria-hidden="true" />
            <span>Edit</span>
            <KeyHint keys="E" />
          </Button>
          <template v-if="!borrowing">
            <Button label="Retire" icon="pi pi-ban" severity="secondary" outlined @click="actions.askRetire(asset)" />
            <Button label="Replace…" icon="pi pi-sync" severity="secondary" outlined @click="replaceOpen = true" />
          </template>
        </template>
        <!-- tài sản bị thay: khôi phục là hoàn tác thay thế -->
        <Button v-else-if="asset.replaced_by" label="Undo replacement" icon="pi pi-undo" @click="actions.undoReplacement(asset)" />
        <Button v-else label="Restore" icon="pi pi-replay" @click="actions.restore(asset)" />
      </template>
      <template v-if="heldTicket && ticketActions.canManage.value">
        <Button label="Hand over" icon="pi pi-arrow-right-arrow-left" @click="ticketActions.doHandOver(heldTicket)" />
        <Button label="Cancel reservation" severity="secondary" outlined @click="ticketActions.doCancel(heldTicket)" />
      </template>
      <Button v-if="canReturn" label="Record return…" icon="pi pi-reply" @click="returnOpen = true" />
      <Button v-if="canLend" label="Lend…" icon="pi pi-arrow-right-arrow-left" severity="secondary" outlined @click="lendOpen = true" />
      <Button v-if="canAsk" label="Borrow this…" icon="pi pi-ticket" severity="secondary" outlined @click="askOpen = true" />
    </div>

    <!-- Details | Activity: lịch sử hoạt động của tài sản -->
    <Tabs v-model:value="tab">
      <TabList>
        <Tab value="details">Details</Tab>
        <Tab value="activity">Activity</Tab>
      </TabList>
      <TabPanels class="panels">
        <TabPanel value="details">
          <div class="cards">
            <Panel header="Asset">
              <dl class="props">
                <dt>Tag</dt>
                <dd class="mono">{{ asset.tag }}</dd>
                <dt>Type</dt>
                <dd>
                  <RouterLink :to="typeListLocation(asset.asset_type.id, listContext.views)">{{ asset.asset_type.name }}</RouterLink>
                </dd>
                <dt>Status</dt>
                <dd>{{ asset.status.name }}</dd>
                <dt>Description</dt>
                <dd class="pre">{{ asset.description || '—' }}</dd>
                <dt>Purchased</dt>
                <dd>{{ formatDate(asset.purchase_date) }}</dd>
                <template v-if="asset.retired_at">
                  <dt>Retired</dt>
                  <dd>{{ formatDateTime(asset.retired_at) }}</dd>
                  <dt>Retired because</dt>
                  <dd>{{ asset.retired_reason || '—' }}</dd>
                </template>
                <dt>Created</dt>
                <dd>{{ formatDateTime(asset.created_at) }}</dd>
                <dt>Updated</dt>
                <dd>{{ formatDateTime(asset.updated_at) }}</dd>
              </dl>
            </Panel>
            <Panel :header="`${asset.asset_type.name} attributes`">
              <dl v-if="asset.attributes.length" class="props">
                <template v-for="a in asset.attributes" :key="a.key">
                  <dt>{{ a.label }}</dt>
                  <dd class="pre">{{ formatValue(a) }}</dd>
                </template>
              </dl>
              <p v-else class="muted">
                This type has no attributes yet.
                <RouterLink v-if="canManageTypes" :to="`/types/${asset.asset_type.id}/settings`">Add some in the type's settings</RouterLink>
              </p>
            </Panel>
            <!-- đang giữ chỗ hay đang cho mượn (US-10) -->
            <Panel v-if="borrowing" header="Borrowing">
              <dl class="props">
                <template v-if="borrowing.state === 'reserved'">
                  <dt>Reserved for</dt>
                  <dd>
                    {{ borrowing.account.name }}
                    <template v-if="borrowing.ticket">
                      · <RouterLink v-if="seeTicket" :to="`/borrowing/tickets/${borrowing.ticket.id}`">ticket #{{ borrowing.ticket.number }}</RouterLink>
                      <span v-else>ticket #{{ borrowing.ticket.number }}</span>
                    </template>
                  </dd>
                </template>
                <template v-else>
                  <dt>On loan to</dt>
                  <dd>
                    {{ borrowing.account.name }}
                    <template v-if="borrowing.ticket">
                      · <RouterLink v-if="seeTicket" :to="`/borrowing/tickets/${borrowing.ticket.id}`">ticket #{{ borrowing.ticket.number }}</RouterLink>
                      <span v-else>ticket #{{ borrowing.ticket.number }}</span>
                    </template>
                  </dd>
                  <dt>Since</dt>
                  <dd>{{ formatDate(borrowing.since) }}</dd>
                </template>
                <dt>Return by</dt>
                <dd>
                  {{ borrowing.return_by ? formatDate(borrowing.return_by) : 'Not set' }}
                  <Tag v-if="borrowing.overdue_days > 0" :value="overdueLabel(borrowing.overdue_days)" severity="danger" class="overdue" />
                </dd>
              </dl>
            </Panel>
            <!-- liên kết thay thế: lần ngược nguồn gốc qua "Replaces" -->
            <Panel v-if="asset.replaced_by || asset.replaces.length" header="Replacement">
              <dl class="props">
                <template v-if="asset.replaced_by">
                  <dt>Replaced by</dt>
                  <dd>
                    <RouterLink :to="`/assets/${asset.replaced_by.id}`" class="mono">{{ asset.replaced_by.tag }}</RouterLink>
                    · {{ asset.replaced_by.name }} · {{ formatDate(asset.replaced_by.replaced_on) }}
                    <div class="pre muted">{{ asset.replaced_by.reason }}</div>
                  </dd>
                </template>
                <template v-if="asset.replaces.length">
                  <dt>Replaces</dt>
                  <dd>
                    <div v-for="r in asset.replaces" :key="r.id">
                      <RouterLink :to="`/assets/${r.id}`" class="mono">{{ r.tag }}</RouterLink>
                      · {{ r.name }} · {{ formatDate(r.replaced_on) }}
                    </div>
                  </dd>
                </template>
              </dl>
            </Panel>
          </div>
        </TabPanel>
        <TabPanel value="activity">
          <AssetActivity v-if="tab === 'activity'" :asset-id="asset.id" />
        </TabPanel>
      </TabPanels>
    </Tabs>

    <RetireDialog v-model:visible="actions.retireOpen.value" :asset="actions.retireTarget.value" />
    <!-- luôn gắn (không v-if theo retired): thay xong tài sản retire ngay, hộp bị gỡ trước khi
         kịp báo đóng thì replaceOpen kẹt true và hộp tự mở lại khi Undo -->
    <ReplaceDialog v-model:visible="replaceOpen" :asset="asset" :type-id="asset.asset_type.id" />
    <LendDialog v-model:visible="lendOpen" :asset="asset" />
    <ReturnDialog v-model:visible="returnOpen" :asset="returnTarget" />
    <RejectDialog v-model:visible="ticketActions.rejectOpen.value" :ticket="ticketActions.rejectTarget.value" :cancel="ticketActions.rejectCancel.value" />
    <NewTicketDialog v-model:visible="askOpen" :preset-type-id="asset.asset_type.id" :suggested="{ id: asset.id, tag: asset.tag }" />
  </section>
</template>

<style scoped>
.overdue {
  margin-left: 0.4rem;
}
.pre {
  white-space: pre-wrap;
}
.asset-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.6rem;
}
.tag-no {
  font: 500 1rem var(--app-mono);
  color: var(--p-text-muted-color);
}
.asset-title :deep(.p-tag) {
  font-family: var(--app-body);
  font-size: 0.75rem;
}
.page-actions {
  margin-bottom: 0.75rem;
}
.page-actions kbd {
  margin-left: 0.25rem;
  font-size: 0.7rem;
}
.panels {
  padding: 1rem 0 0;
  background: transparent;
}
.cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 20rem), 1fr));
  gap: 1rem;
  align-items: start;
}
.muted {
  color: var(--p-text-muted-color);
  margin: 0;
}
.stepper {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  font-variant-numeric: tabular-nums;
  margin-right: 0.5rem;
}
</style>
