<script setup lang="ts">
import Button from 'primevue/button'
import Column from 'primevue/column'
import ContextMenu from 'primevue/contextmenu'
import DataTable from 'primevue/datatable'
import InputText from 'primevue/inputtext'
import Tag from 'primevue/tag'
import { computed, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import EmptyState from '@/components/EmptyState.vue'
import FormDialog from '@/components/FormDialog.vue'
import IconAction from '@/components/IconAction.vue'
import InlineCell from '@/components/InlineCell.vue'
import OverviewFields, { type FieldDef } from '@/components/OverviewFields.vue'
import PageHeader from '@/components/PageHeader.vue'
import QuickEditDrawer from '@/components/QuickEditDrawer.vue'
import SegmentedFilter, { type SegmentOption } from '@/components/SegmentedFilter.vue'
import TableSkeleton from '@/components/TableSkeleton.vue'
import { useCreateExportProfile, useExportProfiles } from '@/features/assets/export/api'
import { defaultReportLayout } from '@/features/assets/export/layout'
import { useExport } from '@/features/assets/export/useExport'
import type { ExportProfile } from '@/lib/api/types'
import { useSession } from '@/lib/auth/session'
import { mayClose } from '@/lib/confirm'
import { useQuickDrawer } from '@/lib/quickDrawer'
import { formatDay } from '@/lib/dates'
import { changesOf, clearTab, emptyDraft, isDirty } from '@/lib/detailDraft'
import { useDirty, useFormErrors } from '@/lib/forms'
import { openLocation } from '@/lib/navigation'
import { onRowClick, useRowMenu } from '@/lib/tableRows'
import { useProfileDelete, useProfileOverviewSave, useProfileShare } from '../overviewSave'

// Profile export: của mình và được chia sẻ. Bấm dòng mở trang profile; bút chì cạnh tên để đổi
// tại chỗ; nút thanh trượt (pi-sliders-h) mở ngăn kéo sửa nhanh; xuất, chia sẻ, xoá ngay trên dòng (có Undo)
const session = useSession()
const router = useRouter()
const { data: profiles, isFetching, isLoading } = useExportProfiles(true)

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

const openProfile = (p: ExportProfile, e?: MouseEvent, newTab?: boolean) => openLocation(router, `/export-profiles/${p.id}`, e, newTab)
const { run } = useExport()
function exportAll(p: ExportProfile) {
  run({ mode: 'report', filters: {}, profile_id: p.id }, `${p.name}.xlsx`)
}
const saveProfile = useProfileOverviewSave()
const toggleShare = useProfileShare()
const deleteProfile = useProfileDelete()
const rename = (p: ExportProfile, name: string) => saveProfile(p, { name })
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

// Bấm dòng mở trang profile; menu chuột phải có cùng các hành động như nút ở cuối dòng
const rowClick = onRowClick<ExportProfile>((p, e) => openProfile(p, e))
const menu = ref<InstanceType<typeof ContextMenu>>()
const { items: menuItems, show: showMenu, clear: clearMenu } = useRowMenu<ExportProfile>(menu, (p) => [
  { label: 'Open', icon: 'pi pi-arrow-right', command: () => openProfile(p) },
  { label: 'Open in new tab', icon: 'pi pi-external-link', command: () => openProfile(p, undefined, true) },
  { label: 'Export all assets', icon: 'pi pi-download', command: () => exportAll(p) },
  { separator: true },
  { label: 'Quick edit', icon: 'pi pi-sliders-h', disabled: !p.can_edit, command: () => openQuick(p) },
  { label: p.shared ? 'Make private' : 'Share', icon: p.shared ? 'pi pi-lock' : 'pi pi-share-alt', disabled: !p.can_edit, command: () => toggleShare(p) },
  { label: 'Delete', icon: 'pi pi-trash', disabled: !p.can_edit, command: () => deleteProfile(p) },
])

// Sửa nhanh: ngăn kéo với tên, chủ (khoá); bên dưới là nút xuất, chia sẻ, xoá như cuối dòng
const quick = ref<ExportProfile | null>(null)
const quickDraft = reactive(emptyDraft())
const quickSaving = ref(false)
const quickFields: FieldDef[] = [
  { key: 'name', label: 'Name', maxlength: 100 },
  { key: 'owner', label: 'Owner', lock: 'Profiles keep the person who made them.' },
]
const quickSaved = computed(() => ({ name: quick.value?.name ?? '', owner: quick.value?.owner.name ?? '' }))
// mở/đóng ngăn kéo: mục giữ lại đến khi trượt ra xong rồi mới xoá cùng bản nháp
const quickDrawer = useQuickDrawer(quick, () => clearTab(quickDraft, 'overview'))
const quickOpen = quickDrawer.visible
function openQuick(p: ExportProfile) {
  quickDrawer.open(p)
}
const quickIndex = computed(() => (quick.value ? visible.value.findIndex((x) => x.id === quick.value!.id) : -1))
async function moveQuick(step: number) {
  const next = visible.value[quickIndex.value + step]
  if (!next || !(await mayClose(isDirty(quickDraft)))) return
  openQuick(next)
}
async function saveQuick() {
  const p = quick.value
  if (!p) return
  quickSaving.value = true
  try {
    if (await saveProfile(p, changesOf(quickDraft, 'overview'))) clearTab(quickDraft, 'overview')
  } finally {
    quickSaving.value = false
  }
}
watch(profiles, (list) => {
  if (quick.value) quick.value = list?.find((x) => x.id === quick.value!.id) ?? null
})

// Profile mới: chỉ hỏi tên, bố cục mặc định; mở ở Columns để chọn cột
const creating = ref(false)
const newName = ref('')
const errors = useFormErrors()
const form = useDirty(() => newName.value.trim())
const create = useCreateExportProfile()
function openCreate() {
  newName.value = ''
  errors.clear()
  form.reset()
  creating.value = true
}
async function submitCreate() {
  errors.clear()
  try {
    const np = await create.mutateAsync({ name: newName.value.trim(), shared: false, layout: defaultReportLayout() })
    creating.value = false
    await router.push({ path: `/export-profiles/${np.id}`, query: { tab: 'columns' } })
  } catch (err) {
    errors.set(err)
  }
}
</script>

<template>
  <section>
    <PageHeader title="Export profiles" subtitle="Saved report layouts. Shared ones can be used by everyone who can export.">
      <Button label="New profile" icon="pi pi-plus" @click="openCreate" />
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
          <InlineCell :value="p.name" label="name" :editable="p.can_edit" class="name" @save="(v) => rename(p, v)">
            {{ p.name }}
          </InlineCell>
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
            <IconAction icon="pi pi-sliders-h" label="Quick edit" :disabled="!p.can_edit" :reason="why(p)" @click="openQuick(p)" />
            <IconAction
              :icon="p.shared ? 'pi pi-lock' : 'pi pi-share-alt'"
              :label="p.shared ? 'Make private' : 'Share'"
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
        <EmptyState v-else icon="pi pi-file-export" text="No profiles yet. Make one here, or save one from Export report… on any asset list." />
      </template>
    </DataTable>

    <QuickEditDrawer
      v-if="quick"
      v-model:visible="quickOpen"
      @closed="quickDrawer.closed()"
      :title="quick.name"
      icon="file"
      :dirty="isDirty(quickDraft)"
      :busy="quickSaving"
      :can-prev="quickIndex > 0"
      :can-next="quickIndex >= 0 && quickIndex < visible.length - 1"
      actions-label="Profile"
      @save="saveQuick"
      @prev="moveQuick(-1)"
      @next="moveQuick(1)"
      @open-page="router.push(`/export-profiles/${quick.id}`)"
    >
      <OverviewFields :fields="quickFields" :saved="quickSaved" :draft="quickDraft" :readonly="!quick.can_edit" stacked />
      <!-- hành động như nút cuối dòng; chia sẻ / làm riêng tư hỏi trước -->
      <template #actions>
        <Button label="Export all assets" icon="pi pi-download" severity="secondary" outlined size="small" @click="exportAll(quick)" />
        <template v-if="quick.can_edit">
          <Button
            :label="quick.shared ? 'Make private' : 'Share'"
            :icon="quick.shared ? 'pi pi-lock' : 'pi pi-share-alt'"
            severity="secondary"
            outlined
            size="small"
            @click="toggleShare(quick)"
          />
          <Button label="Delete" icon="pi pi-trash" severity="danger" outlined size="small" @click="deleteProfile(quick)" />
        </template>
      </template>
    </QuickEditDrawer>

    <FormDialog
      v-model:visible="creating"
      size="s"
      icon="file"
      title="New export profile"
      action="Create profile"
      :busy="create.isPending.value"
      :error="errors.general.value"
      :dirty="form.dirty.value"
      @submit="submitCreate"
    >
      <div class="field">
        <label for="profile-name">Name</label>
        <InputText id="profile-name" v-model="newName" required maxlength="100" autofocus />
        <small v-if="errors.fields.value.name" class="field-error">{{ errors.fields.value.name }}</small>
      </div>
      <template #hint>Starts with Tag, Name, Type, Status and Purchase date. Pick columns on the next page.</template>
    </FormDialog>
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
