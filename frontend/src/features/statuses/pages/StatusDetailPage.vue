<script setup lang="ts">
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useLeaveGuard, useTabDirty, useTabTitle } from '@/app/tabs/tabPage'
import AppBreadcrumb from '@/components/AppBreadcrumb.vue'
import DetailHeader from '@/components/DetailHeader.vue'
import EmptyState from '@/components/EmptyState.vue'
import OverviewFields, { type FieldDef } from '@/components/OverviewFields.vue'
import SaveBar from '@/components/SaveBar.vue'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { changeCount, changesOf, clearTab, discardTab, emptyDraft, isDirty, restoreTab } from '@/lib/detailDraft'
import { notify } from '@/lib/notify'
import { kindSeverity, useStatuses } from '../api'
import { archiveBlock, KIND_INFO, lanes } from '../lanes'
import { useStatusLifecycle, useStatusOverviewSave } from '../overviewSave'

// Trang của một status: chỉ Overview (tên sửa tại chỗ, kind khoá). Không có API đọc một
// status: lấy từ danh sách (nhỏ, đã có trong cache)
const props = defineProps<{ id: string }>()

const session = useSession()
const canManage = computed(() => session.can(Perm.StatusManage))
const { data: statuses, isLoading } = useStatuses(true, true)
const status = computed(() => statuses.value?.find((s) => s.id === props.id))
useTabTitle(() => status.value?.name)

// vị trí trong làn (chỉ các status đang dùng)
const lane = computed(() => (status.value ? lanes(statuses.value ?? [])[status.value.kind].filter((s) => !s.archived_at) : []))
const position = computed(() => lane.value.findIndex((s) => s.id === props.id) + 1)
const assets = computed(() => status.value?.asset_count ?? 0)
const assetsLink = computed(() => ({
  path: '/assets',
  query: { status_id: props.id, ...(status.value?.kind === 'retired' ? { include_retired: 'true' } : {}) },
}))

const fields: FieldDef[] = [
  { key: 'name', label: 'Name', maxlength: 100 },
  { key: 'kind', label: 'Kind', lock: 'The kind decides how assets behave, so it can’t change.' },
]
const saved = computed(() => ({ name: status.value?.name ?? '', kind: status.value ? KIND_INFO[status.value.kind].label : '' }))
const draft = reactive(emptyDraft())
useTabDirty(() => isDirty(draft))
useLeaveGuard(() => isDirty(draft))
const saveOverview = useStatusOverviewSave()
const saving = ref(false)
async function save() {
  const s = status.value
  if (!s) return
  saving.value = true
  try {
    if (await saveOverview(s, changesOf(draft, 'overview'))) clearTab(draft, 'overview')
  } finally {
    saving.value = false
  }
}
function discard() {
  const removed = discardTab(draft, 'overview')
  notify.success('Changes discarded.', { undo: () => restoreTab(draft, 'overview', removed) })
}

const lifecycle = useStatusLifecycle()

// Mở từ bút chì cũ / "Open" sau khi thêm: ?focus=name chọn sẵn ô tên
const route = useRoute()
watch(
  () => status.value?.id,
  async (id) => {
    if (!id || route.query.focus !== 'name') return
    await nextTick()
    const el = document.getElementById('of-name') as HTMLInputElement | null
    el?.focus()
    el?.select()
  },
  { immediate: true },
)
const crumbs = computed(() => [{ label: 'Statuses', to: '/statuses' }, { label: status.value?.name ?? '…' }])
</script>

<template>
  <section v-if="status">
    <AppBreadcrumb :items="crumbs" />
    <DetailHeader :title="status.name" icon="tag">
      <template #tags>
        <Tag :value="KIND_INFO[status.kind].label" :severity="kindSeverity(status.kind)" />
        <Tag v-if="status.is_default" value="Default" severity="success" />
        <Tag v-if="status.archived_at" value="Archived" severity="secondary" />
      </template>
      <div>
        {{ assets }} {{ assets === 1 ? 'asset' : 'assets' }}<template v-if="position"> · {{ position }} of {{ lane.length }} in {{ KIND_INFO[status.kind].label }}</template>
      </div>
      <template #actions>
        <Button v-slot="slot" as-child text>
          <RouterLink :to="assetsLink" :class="slot.class">View {{ assets }} {{ assets === 1 ? 'asset' : 'assets' }} <i class="pi pi-arrow-right" aria-hidden="true" /></RouterLink>
        </Button>
        <template v-if="canManage">
          <Button
            v-if="!status.is_default && !status.archived_at && status.kind !== 'retired'"
            label="Make default"
            severity="secondary"
            outlined
            @click="lifecycle.makeDefault(status, statuses ?? [])"
          />
          <Button v-if="status.archived_at" label="Restore" severity="secondary" outlined @click="lifecycle.restore(status)" />
          <span v-else v-tooltip.top="archiveBlock(status)">
            <Button label="Archive" severity="secondary" outlined :disabled="!!archiveBlock(status)" @click="lifecycle.archive(status)" />
          </span>
        </template>
      </template>
    </DetailHeader>

    <OverviewFields :fields="fields" :saved="saved" :draft="draft" :readonly="!canManage" />
    <SaveBar v-if="canManage && changeCount(draft, 'overview')" :count="changeCount(draft, 'overview')" :saving="saving" @save="save" @discard="discard" />
  </section>
  <EmptyState v-else-if="!isLoading" icon="pi pi-tag" text="This status doesn't exist." />
</template>
