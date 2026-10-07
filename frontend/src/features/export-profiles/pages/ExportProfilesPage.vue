<script setup lang="ts">
import Button from 'primevue/button'
import Column from 'primevue/column'
import ContextMenu from 'primevue/contextmenu'
import DataTable from 'primevue/datatable'
import InputText from 'primevue/inputtext'
import Tag from 'primevue/tag'
import { computed, nextTick, ref } from 'vue'
import EmptyState from '@/components/EmptyState.vue'
import TableSkeleton from '@/components/TableSkeleton.vue'
import IconAction from '@/components/IconAction.vue'
import PageHeader from '@/components/PageHeader.vue'
import SegmentedFilter, { type SegmentOption } from '@/components/SegmentedFilter.vue'
import { useAssetTypes } from '@/features/asset-types/api'
import { useDeleteExportProfile, useExportProfiles, useRestoreExportProfile, useUpdateExportProfile } from '@/features/assets/export/api'
import ReportDialog from '@/features/assets/export/components/ReportDialog.vue'
import { useExport } from '@/features/assets/export/useExport'
import type { ExportScope } from '@/features/assets/export/usePreviewData'
import type { ExportProfile } from '@/lib/api/types'
import { useSession } from '@/lib/auth/session'
import { runAction } from '@/lib/actions'
import { formatDay } from '@/lib/dates'
import { onRowClick, useRowMenu } from '@/lib/tableRows'

// Profile export: của mình và được chia sẻ; mở để sửa, chạy trên mọi tài sản, chia sẻ, xoá
const session = useSession()
const { data: profiles, isFetching, isLoading } = useExportProfiles(true)
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
const update = useUpdateExportProfile(false)
const remove = useDeleteExportProfile()
const restore = useRestoreExportProfile()
function exportAll(p: ExportProfile) {
  run({ mode: 'report', filters: {}, profile_id: p.id }, `${p.name}.xlsx`)
}
// Đổi tên ngay trên dòng: ô tên thành ô nhập, Enter lưu, Escape huỷ
const renaming = ref<string | null>(null)
const newName = ref('')
function startRename(p: ExportProfile) {
  renaming.value = p.id
  newName.value = p.name
  nextTick(() => (document.getElementById(`rename-${p.id}`) as HTMLInputElement | null)?.select())
}
function cancelRename() {
  renaming.value = null
}
async function saveRename(p: ExportProfile) {
  const name = newName.value.trim()
  if (!name || name === p.name) {
    cancelRename()
    return
  }
  const ok = await runAction({
    run: () => update.mutateAsync({ id: p.id, version: p.version, name }),
    done: `${p.name} renamed to ${name}.`,
    failed: `Couldn't rename ${p.name}.`,
    undo: (next) => update.mutateAsync({ id: p.id, version: next.version, name: p.name }),
    undone: `Name put back to ${p.name}.`,
    undoFailed: `Couldn't put the name back. It stays ${name}.`,
  })
  // lỗi (trùng tên, đã bị sửa): giữ ô nhập để sửa tiếp
  if (ok) renaming.value = null
}

function toggleShare(p: ExportProfile) {
  const sharing = !p.shared
  return runAction({
    run: () => update.mutateAsync({ id: p.id, version: p.version, shared: sharing }),
    done: sharing ? `${p.name} is shared with everyone who can export.` : `${p.name} is private again.`,
    failed: `Couldn't change who sees ${p.name}.`,
    undo: (next) => update.mutateAsync({ id: p.id, version: next.version, shared: p.shared }),
    undone: sharing ? `${p.name} is private again.` : `${p.name} is shared again.`,
    undoFailed: `Couldn't change who sees ${p.name} back.`,
  })
}
function deleteProfile(p: ExportProfile) {
  return runAction({
    run: () => remove.mutateAsync(p.id),
    done: `${p.name} deleted.`,
    failed: `Couldn't delete ${p.name}.`,
    undo: () => restore.mutateAsync(p.id),
    undone: `${p.name} restored.`,
    undoFailed: `Couldn't restore ${p.name}. It stays deleted.`,
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
  { label: p.can_edit ? 'Edit layout' : 'Open (save as a copy)', icon: 'pi pi-sliders-h', command: () => open(p) },
  { label: 'Rename…', icon: 'pi pi-pencil', disabled: !p.can_edit, command: () => startRename(p) },
  { label: 'Export all assets', icon: 'pi pi-download', command: () => exportAll(p) },
  { separator: true },
  {
    label: p.shared ? 'Stop sharing' : 'Share',
    icon: p.shared ? 'pi pi-lock' : 'pi pi-share-alt',
    disabled: !p.can_edit,
    command: () => toggleShare(p),
  },
  { label: 'Delete', icon: 'pi pi-trash', disabled: !p.can_edit, command: () => deleteProfile(p) },
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
          <form v-if="renaming === p.id" class="rename" @submit.prevent="saveRename(p)" @keydown.esc.prevent="cancelRename">
            <InputText :id="`rename-${p.id}`" v-model="newName" size="small" maxlength="100" required :aria-label="`New name for ${p.name}`" />
            <Button type="submit" label="Save" size="small" :loading="update.isPending.value" />
            <Button label="Cancel" size="small" text severity="secondary" @click="cancelRename" />
          </form>
          <span v-else class="name">{{ p.name }}</span>
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
      <Column header="" header-style="width: 12rem">
        <template #body="{ data: p }: { data: ExportProfile }">
          <div class="row-actions">
            <IconAction icon="pi pi-download" label="Export all assets with this profile" @click="exportAll(p)" />
            <IconAction icon="pi pi-sliders-h" :label="p.can_edit ? 'Edit layout' : 'Open (save as a copy)'" @click="open(p)" />
            <IconAction icon="pi pi-pencil" label="Rename" :disabled="!p.can_edit" :reason="why(p)" @click="startRename(p)" />
            <IconAction
              :icon="p.shared ? 'pi pi-lock' : 'pi pi-share-alt'"
              :label="p.shared ? 'Stop sharing' : 'Share'"
              :disabled="!p.can_edit"
              :reason="why(p)"
              @click="toggleShare(p)"
            />
            <IconAction icon="pi pi-trash" label="Delete" danger :disabled="!p.can_edit" :reason="why(p)" @click="deleteProfile(p)" />
          </div>
        </template>
      </Column>
      <template #empty>
        <TableSkeleton v-if="isLoading" />
        <EmptyState v-else icon="pi pi-file-export" text="No profiles yet. Save one from Export report… on any asset list." />
      </template>
    </DataTable>
    <ReportDialog v-model:visible="dialogOpen" :scope="scope" :profile-id="dialogProfile" />
  </section>
</template>

<style scoped>
.name {
  font-weight: 600;
}
.rename {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem;
}
.rename :deep(.p-inputtext) {
  min-width: 12rem;
}
.muted {
  color: var(--p-text-muted-color);
}
</style>
