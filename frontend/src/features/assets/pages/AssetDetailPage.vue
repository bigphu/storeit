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
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useTabId, useTabTitle } from '@/app/tabs/tabPage'
import AppBreadcrumb, { type Crumb } from '@/components/AppBreadcrumb.vue'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { formatDate, formatDateTime } from '@/lib/dates'
import { usePageKeys } from '@/lib/pageKeys'
import { kindSeverity } from '@/features/statuses/api'
import { fetchAssetPage, useAsset } from '../api'
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
const retired = computed(() => !!asset.value?.retired_at)
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

    <div v-if="canManage" class="actions page-actions">
      <template v-if="!retired">
        <Button @click="(e: MouseEvent) => actions.edit(asset!, e)">
          <i class="pi pi-pencil" />
          <span>Edit</span>
          <kbd>E</kbd>
        </Button>
        <Button label="Retire" icon="pi pi-ban" severity="secondary" outlined @click="actions.askRetire(asset)" />
      </template>
      <Button v-else label="Restore" icon="pi pi-replay" @click="actions.restore(asset)" />
    </div>

    <!-- Details | Activity: lịch sử hoạt động là module sau -->
    <Tabs value="details">
      <TabList>
        <Tab value="details">Details</Tab>
        <Tab value="activity" disabled>Activity <small class="later">later</small></Tab>
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
          </div>
        </TabPanel>
      </TabPanels>
    </Tabs>

    <RetireDialog v-model:visible="actions.retireOpen.value" :asset="actions.retireTarget.value" />
  </section>
</template>

<style scoped>
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
.later {
  font-weight: 400;
  font-style: italic;
  color: var(--p-text-muted-color);
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
