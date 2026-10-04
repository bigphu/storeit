<script setup lang="ts">
import { useQueryClient } from '@tanstack/vue-query'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useTabId, useTabTitle } from '@/app/tabs/tabPage'
import AppBreadcrumb, { type Crumb } from '@/components/AppBreadcrumb.vue'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { formatDate, formatDateTime } from '@/lib/dates'
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

// Phím tắt: J/K bước qua danh sách, E sửa
function onKey(e: KeyboardEvent) {
  const t = e.target as HTMLElement | null
  if (e.ctrlKey || e.metaKey || e.altKey || (t && /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName)) || t?.isContentEditable) return
  if (document.querySelector('.p-dialog-mask')) return
  if (e.key === 'j') step(1)
  else if (e.key === 'k') step(-1)
  else if (e.key === 'e' && canManage.value && asset.value && !retired.value) actions.edit(asset.value)
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <section v-if="asset">
    <AppBreadcrumb :items="crumbs" />
    <div class="page-header">
      <h1>{{ asset.tag }} — {{ asset.name }}</h1>
      <div class="actions">
        <span v-if="pos" class="stepper" aria-label="Position in the list">
          <Button icon="pi pi-angle-left" text rounded aria-label="Previous asset (K)" :disabled="pos.index === 0" @click="step(-1)" />
          <span>{{ pos.index + 1 }} of {{ pos.total }}</span>
          <Button icon="pi pi-angle-right" text rounded aria-label="Next asset (J)" :disabled="pos.index + 1 >= pos.total" @click="step(1)" />
        </span>
        <template v-if="canManage">
          <Button v-if="!retired" label="Edit" icon="pi pi-pencil" @click="(e: MouseEvent) => actions.edit(asset!, e)" />
          <Button v-if="!retired" label="Retire" severity="danger" text @click="actions.askRetire(asset)" />
          <Button v-else label="Restore" @click="actions.askRestore(asset)" />
        </template>
      </div>
    </div>

    <dl class="props">
      <dt>Type</dt>
      <dd>
        <RouterLink :to="typeListLocation(asset.asset_type.id, listContext.views)">{{ asset.asset_type.name }}</RouterLink>
      </dd>
      <dt>Status</dt>
      <dd><Tag :value="asset.status.name" :severity="kindSeverity(asset.status.kind)" /></dd>
      <dt>Description</dt>
      <dd class="pre">{{ asset.description || '—' }}</dd>
      <dt>Purchase date</dt>
      <dd>{{ formatDate(asset.purchase_date) }}</dd>
      <template v-if="asset.retired_at">
        <dt>Retired</dt>
        <dd>{{ formatDateTime(asset.retired_at) }}<template v-if="asset.retired_reason"> — {{ asset.retired_reason }}</template></dd>
      </template>
      <dt>Created</dt>
      <dd>{{ formatDateTime(asset.created_at) }}</dd>
      <dt>Updated</dt>
      <dd>{{ formatDateTime(asset.updated_at) }}</dd>
    </dl>

    <section>
      <h2>Attributes</h2>
      <dl v-if="asset.attributes.length" class="props">
        <template v-for="a in asset.attributes" :key="a.key">
          <dt>{{ a.label }}</dt>
          <dd class="pre">{{ formatValue(a) }}</dd>
        </template>
      </dl>
      <p v-else>This asset type has no attributes.</p>
    </section>

    <RetireDialog v-model:visible="actions.retireOpen.value" :asset="actions.retireTarget.value" />
  </section>
</template>

<style scoped>
.pre {
  white-space: pre-wrap;
}
.stepper {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  font-variant-numeric: tabular-nums;
  margin-right: 0.5rem;
}
</style>
