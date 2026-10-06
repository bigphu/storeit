<script setup lang="ts">
import Button from 'primevue/button'
import Column from 'primevue/column'
import ContextMenu from 'primevue/contextmenu'
import DataTable from 'primevue/datatable'
import Tag from 'primevue/tag'
import { useConfirm } from 'primevue/useconfirm'
import { computed, ref } from 'vue'
import IconAction from '@/components/IconAction.vue'
import PageHeader from '@/components/PageHeader.vue'
import SegmentedFilter, { type SegmentOption } from '@/components/SegmentedFilter.vue'
import { useAssetTypes } from '@/features/asset-types/api'
import { useDeleteExportProfile, useExportProfiles, useUpdateExportProfile } from '@/features/assets/export/api'
import ReportDialog from '@/features/assets/export/components/ReportDialog.vue'
import { useExport } from '@/features/assets/export/useExport'
import type { ExportScope } from '@/features/assets/export/usePreviewData'
import type { ExportProfile } from '@/lib/api/types'
import { useSession } from '@/lib/auth/session'
import { formatDay } from '@/lib/dates'
import { notify } from '@/lib/notify'
import { onRowClick, useRowMenu } from '@/lib/tableRows'

// Profile export: của mình và được chia sẻ; mở để sửa, chạy trên mọi tài sản, chia sẻ, xoá
const session = useSession()
const confirm = useConfirm()
const { data: profiles, isFetching } = useExportProfiles(true)
const { data: types } = useAssetTypes(false, true)

type Show = 'all' | 'mine' | 'shared'
const show = ref<Show>('all')
const mine = (p: ExportProfile) => p.owner.id === session.me?.account.id
const showOptions = computed<SegmentOption<Show>[]>(() => [
  { label: 'All', value: 'all', count: profiles.value?.length ?? 0 },
  { label: 'Mine', value: 'mine', count: profiles.value?.filter(mine).length ?? 0 },
  { label: 'Shared', value: 'shared', count: profiles.value?.filter((p) => p.shared).length ?? 0 },
])
const visible = computed(() =>
  (profiles.value ?? []).filter((p) => show.value === 'all' || (show.value === 'mine' ? mine(p) : p.shared)),
)

// phạm vi khi mở từ trang này: mọi tài sản
const scope = computed<ExportScope>(() => ({
  filters: {},
  label: 'All assets',
  count: (types.value ?? []).reduce((n, t) => n + (t.asset_count ?? 0), 0),
  typeIds: (types.value ?? []).filter((t) => (t.asset_count ?? 0) > 0).map((t) => t.id),
  rows: [],
  selection: false,
}))
const dialogOpen = ref(false)
const dialogProfile = ref<string | undefined>()
function open(p?: ExportProfile) {
  dialogProfile.value = p?.id
  dialogOpen.value = true
}

const { run } = useExport()
const update = useUpdateExportProfile()
const remove = useDeleteExportProfile()
function exportAll(p: ExportProfile) {
  run({ mode: 'report', filters: {}, profile_id: p.id }, `${p.name}.xlsx`)
}
function toggleShare(p: ExportProfile) {
  update.mutateAsync({ id: p.id, version: p.version, shared: !p.shared }).then(
    () => notify.success(p.shared ? `${p.name} is private again.` : `${p.name} is shared with everyone who can export.`),
    () => {},
  )
}
function askDelete(p: ExportProfile) {
  confirm.require({
    header: 'Delete profile',
    message: `Delete ${p.name}?${p.shared ? ' People who use this shared profile lose it too.' : ''}`,
    acceptLabel: 'Delete',
    rejectLabel: 'Cancel',
    acceptProps: { severity: 'danger' },
    accept: () => remove.mutateAsync(p.id).then(() => notify.success(`Deleted ${p.name}.`), () => {}),
  })
}
const why = (p: ExportProfile) => `Only ${p.owner.name} or a profile manager can change this`
function summary(p: ExportProfile) {
  const n = p.layout.columns.length
  return [
    `${n} column${n === 1 ? '' : 's'}`,
    p.layout.sheets === 'per_type' ? 'sheet per type' : 'one sheet',
    p.layout.title_row && 'title row',
    p.layout.summary && 'summary',
  ]
    .filter(Boolean)
    .join(' · ')
}

// Bấm dòng mở profile; menu chuột phải có cùng các hành động như nút ở cuối dòng
const rowClick = onRowClick<ExportProfile>((p) => open(p))
const menu = ref<InstanceType<typeof ContextMenu>>()
const { items: menuItems, show: showMenu, clear: clearMenu } = useRowMenu<ExportProfile>(menu, (p) => [
  { label: p.can_edit ? 'Edit' : 'Open (save as a copy)', icon: 'pi pi-pencil', command: () => open(p) },
  { label: 'Export all assets', icon: 'pi pi-download', command: () => exportAll(p) },
  { separator: true },
  {
    label: p.shared ? 'Stop sharing' : 'Share',
    icon: p.shared ? 'pi pi-lock' : 'pi pi-share-alt',
    disabled: !p.can_edit,
    command: () => toggleShare(p),
  },
  { label: 'Delete', icon: 'pi pi-trash', disabled: !p.can_edit, command: () => askDelete(p) },
])
</script>

<template>
  <section>
    <PageHeader title="Export profiles" subtitle="Saved report layouts. Shared ones can be used by everyone who can export.">
      <Button label="New profile" icon="pi pi-plus" @click="open()" />
    </PageHeader>
    <div class="toolbar">
      <SegmentedFilter v-model="show" :options="showOptions" label="Show" />
    </div>

    <ContextMenu ref="menu" :model="menuItems" @hide="clearMenu" />

    <DataTable
      :value="visible"
      :loading="isFetching"
      data-key="id"
      row-hover
      :row-class="() => 'clickable-row'"
      @row-click="rowClick"
      @row-contextmenu="showMenu"
    >
      <Column header="Name">
        <template #body="{ data: p }: { data: ExportProfile }">
          <span class="name">{{ p.name }}</span>
        </template>
      </Column>
      <Column header="Owner">
        <template #body="{ data: p }: { data: ExportProfile }">{{ mine(p) ? 'You' : p.owner.name }}</template>
      </Column>
      <Column header="Visibility">
        <template #body="{ data: p }: { data: ExportProfile }">
          <Tag :value="p.shared ? 'Shared' : 'Only you'" :severity="p.shared ? 'info' : 'secondary'" />
        </template>
      </Column>
      <Column header="Layout">
        <template #body="{ data: p }: { data: ExportProfile }"><span class="muted">{{ summary(p) }}</span></template>
      </Column>
      <Column header="Updated">
        <template #body="{ data: p }: { data: ExportProfile }">{{ formatDay(p.updated_at) }}</template>
      </Column>
      <Column header="" header-style="width: 10rem">
        <template #body="{ data: p }: { data: ExportProfile }">
          <div class="row-actions">
            <IconAction icon="pi pi-download" label="Export all assets with this profile" @click="exportAll(p)" />
            <IconAction icon="pi pi-pencil" :label="p.can_edit ? 'Edit' : 'Open (save as a copy)'" @click="open(p)" />
            <IconAction
              :icon="p.shared ? 'pi pi-lock' : 'pi pi-share-alt'"
              :label="p.shared ? 'Stop sharing' : 'Share'"
              :disabled="!p.can_edit"
              :reason="why(p)"
              @click="toggleShare(p)"
            />
            <IconAction icon="pi pi-trash" label="Delete" danger :disabled="!p.can_edit" :reason="why(p)" @click="askDelete(p)" />
          </div>
        </template>
      </Column>
      <template #empty>No profiles yet. Save one from Export report… on any asset list.</template>
    </DataTable>
    <ReportDialog v-model:visible="dialogOpen" :scope="scope" :profile-id="dialogProfile" />
  </section>
</template>

<style scoped>
.name {
  font-weight: 600;
}
.muted {
  color: var(--p-text-muted-color);
}
</style>
