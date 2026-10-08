<script setup lang="ts">
import Button from 'primevue/button'
import Menu from 'primevue/menu'
import type { MenuItem } from 'primevue/menuitem'
import Tab from 'primevue/tab'
import TabList from 'primevue/tablist'
import Tabs from 'primevue/tabs'
import Tag from 'primevue/tag'
import { computed, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useLeaveGuard, useTabDirty, useTabTitle } from '@/app/tabs/tabPage'
import AppBreadcrumb from '@/components/AppBreadcrumb.vue'
import DetailHeader from '@/components/DetailHeader.vue'
import EmptyState from '@/components/EmptyState.vue'
import OverviewFields, { type FieldDef } from '@/components/OverviewFields.vue'
import SaveBar from '@/components/SaveBar.vue'
import { useCreateExportProfile, useDeleteExportProfile, useExportProfiles } from '@/features/assets/export/api'
import ReportEditor from '@/features/assets/export/components/ReportEditor.vue'
import { exportFileName, normalizeLayout } from '@/features/assets/export/layout'
import { useExport } from '@/features/assets/export/useExport'
import { usePreviewData } from '@/features/assets/export/usePreviewData'
import type { ExportLayout } from '@/lib/api/types'
import { runAction } from '@/lib/actions'
import { changeCount, changesOf, clearTab, discardTab, emptyDraft, isDirty, restoreTab } from '@/lib/detailDraft'
import { notify } from '@/lib/notify'
import { useUrlState } from '@/lib/urlState'
import { useAllAssetsScope } from '../allAssetsScope'
import { useProfileDelete, useProfileLayoutSave, useProfileOverviewSave, useProfileShare } from '../overviewSave'

// Trang của một profile export: Overview (tên) | Columns | Layout | Format (trình sửa bố
// cục dùng chung với hộp thoại báo cáo, bản xem trước bên cạnh). Profile người khác chia sẻ
// mà mình không sửa được: chỉ xem, có "Duplicate to change it"
const props = defineProps<{ id: string }>()
const router = useRouter()

const { data: profiles, isLoading } = useExportProfiles(true)
const profile = computed(() => profiles.value?.find((p) => p.id === props.id))
const canEdit = computed(() => !!profile.value?.can_edit)
useTabTitle(() => profile.value?.name)

type Section = 'overview' | 'columns' | 'layout' | 'format'
const SECTIONS: Section[] = ['overview', 'columns', 'layout', 'format']
const { state, update: updateUrl } = useUrlState(
  (q) => ({ tab: (SECTIONS.includes(q.tab as Section) ? q.tab : 'overview') as Section }),
  (s) => ({ tab: s.tab === 'overview' ? undefined : s.tab }),
)
const onLayoutTab = computed(() => state.value.tab !== 'overview')
const editorTab = computed({
  get: () => (onLayoutTab.value ? (state.value.tab as Exclude<Section, 'overview'>) : 'columns'),
  set: (t) => updateUrl({ tab: t }),
})

// Overview: tên sửa tại chỗ, chủ khoá
const fields: FieldDef[] = [
  { key: 'name', label: 'Name', maxlength: 100 },
  { key: 'owner', label: 'Owner', lock: 'Profiles keep the person who made them.' },
]
const saved = computed(() => ({ name: profile.value?.name ?? '', owner: profile.value?.owner.name ?? '' }))
const draft = reactive(emptyDraft())

// Bố cục: trình sửa nạp lại khi version đổi hay khi Discard (layoutKey); editorSource để Undo
// của Discard đưa lại bản đang sửa
const scope = useAllAssetsScope()
const { types, rows } = usePreviewData(
  () => scope.value,
  () => onLayoutTab.value,
)
const layoutKey = ref(0)
const editorSource = ref<ExportLayout | null>(null)
const layoutDraft = ref<ExportLayout | null>(null)
const layoutDirty = computed(() => !!profile.value && !!layoutDraft.value && normalizeLayout(layoutDraft.value) !== normalizeLayout(profile.value.layout))
watch(
  () => profile.value?.version,
  () => (editorSource.value = null),
)

useTabDirty(() => isDirty(draft) || layoutDirty.value)
useLeaveGuard(() => isDirty(draft) || layoutDirty.value)

const saveOverview = useProfileOverviewSave()
const saveLayout = useProfileLayoutSave()
const saving = ref(false)
async function save() {
  const p = profile.value
  if (!p) return
  saving.value = true
  try {
    if (!onLayoutTab.value) {
      if (await saveOverview(p, changesOf(draft, 'overview'))) clearTab(draft, 'overview')
    } else if (layoutDraft.value) {
      await saveLayout(p, layoutDraft.value)
    }
  } finally {
    saving.value = false
  }
}
function discard() {
  if (!onLayoutTab.value) {
    const removed = discardTab(draft, 'overview')
    notify.success('Changes discarded.', { undo: () => restoreTab(draft, 'overview', removed) })
    return
  }
  const kept = layoutDraft.value
  editorSource.value = null
  layoutKey.value++
  notify.success('Changes discarded.', {
    undo: () => {
      editorSource.value = kept
      layoutKey.value++
    },
  })
}
const saveBar = computed(() => {
  if (!canEdit.value) return null
  if (!onLayoutTab.value) {
    const n = changeCount(draft, 'overview')
    return n ? { count: n } : null
  }
  return layoutDirty.value ? { message: 'Layout changed' } : null
})

// Hành động: Export now, Share/Make private, More (Duplicate), Delete
const { run: runExport, running } = useExport()
function exportNow() {
  const p = profile.value
  if (p) void runExport({ mode: 'report', filters: {}, profile_id: p.id }, `${p.name}.xlsx`)
}
const share = useProfileShare()
const removeProfile = useProfileDelete()
const create = useCreateExportProfile()
const removeCopy = useDeleteExportProfile()
function duplicate() {
  const p = profile.value
  if (!p) return
  return runAction({
    run: () => create.mutateAsync({ name: `${p.name} (copy)`, shared: false, layout: p.layout }),
    done: (np) => `${np.name} created.`,
    failed: `Couldn't duplicate ${p.name}.`,
    undo: (np) => removeCopy.mutateAsync(np.id),
    undone: 'Copy deleted.',
    after: (np) => router.push({ path: `/export-profiles/${np.id}`, query: { tab: 'columns' } }),
  })
}
const more = ref<InstanceType<typeof Menu>>()
const moreItems = computed<MenuItem[]>(() => [{ label: 'Duplicate', icon: 'pi pi-clone', command: duplicate }])

const facts = computed(() => {
  const p = profile.value
  if (!p) return ''
  const n = p.layout.columns.length
  return `By ${p.owner.name} · ${n} ${n === 1 ? 'column' : 'columns'} · ${p.layout.sheets === 'per_type' ? 'sheet per type' : 'one sheet'}`
})
const fileName = computed(() => exportFileName('report', profile.value?.name, new Date().toISOString().slice(0, 10)))
const crumbs = computed(() => [{ label: 'Export profiles', to: '/export-profiles' }, { label: profile.value?.name ?? '…' }])
</script>

<template>
  <section v-if="profile">
    <AppBreadcrumb :items="crumbs" />
    <DetailHeader :title="profile.name" icon="file">
      <template #tags>
        <Tag :value="profile.shared ? 'Shared' : 'Only you'" :severity="profile.shared ? 'info' : 'secondary'" />
      </template>
      <div>{{ facts }}</div>
      <template #actions>
        <Button label="Export now" icon="pi pi-download" :loading="running" @click="exportNow" />
        <template v-if="canEdit">
          <Button :label="profile.shared ? 'Make private' : 'Share'" severity="secondary" outlined @click="share(profile)" />
          <Button icon="pi pi-ellipsis-h" severity="secondary" outlined aria-label="More actions" aria-haspopup="menu" @click="(e: MouseEvent) => more?.toggle(e)" />
          <Menu ref="more" :model="moreItems" popup />
          <Button label="Delete" icon="pi pi-trash" severity="danger" outlined @click="removeProfile(profile, () => router.push('/export-profiles'))" />
        </template>
        <Button v-else label="Duplicate to change it" icon="pi pi-clone" severity="secondary" outlined @click="duplicate" />
      </template>
    </DetailHeader>

    <Tabs :value="state.tab" class="section-tabs" @update:value="(v) => updateUrl({ tab: v as Section })">
      <TabList>
        <Tab value="overview">Overview<span v-if="changeCount(draft, 'overview')" class="tab-dirty" aria-label="Unsaved changes" /></Tab>
        <Tab value="columns">Columns <span class="tab-count">{{ profile.layout.columns.length }}</span></Tab>
        <Tab value="layout">Layout</Tab>
        <Tab value="format">Format</Tab>
      </TabList>
    </Tabs>

    <OverviewFields v-if="!onLayoutTab" :fields="fields" :saved="saved" :draft="draft" :readonly="!canEdit" />
    <div v-else class="editor-wrap">
      <ReportEditor
        v-model:tab="editorTab"
        hide-tabs
        :source="editorSource ?? profile.layout"
        :source-key="`${profile.version}:${layoutKey}`"
        :types="types"
        :rows="rows"
        :scope-label="scope.label"
        :row-count="scope.count"
        :file-name="fileName"
        :title-name="profile.name"
        @update:current="(l) => (layoutDraft = l)"
      />
    </div>
    <p v-if="onLayoutTab && !canEdit" class="hint">Shared by {{ profile.owner.name }}. Changes here only change the preview; duplicate it to keep them.</p>

    <SaveBar v-if="saveBar" :count="saveBar.count" :message="saveBar.message" :saving="saving" @save="save" @discard="discard" />
  </section>
  <EmptyState v-else-if="!isLoading" icon="pi pi-file-export" text="This export profile doesn't exist or isn't shared with you." />
</template>

<style scoped>
.section-tabs {
  margin-bottom: 1rem;
}
.tab-count {
  font: 0.75rem var(--app-mono);
  color: var(--p-text-muted-color);
}
.tab-dirty {
  display: inline-block;
  width: 0.45rem;
  height: 0.45rem;
  margin-left: 0.4rem;
  border-radius: 50%;
  background: var(--app-warn);
}
/* trình sửa có hai cột tự cuộn: cho nó chiều cao cố định */
.editor-wrap {
  display: flex;
  flex-direction: column;
  height: min(44rem, 72vh);
  border: 1px solid var(--app-line);
  border-radius: 12px;
  overflow: hidden;
  background: var(--p-content-background);
}
.hint {
  margin: 0.6rem 0 0;
  font-size: 0.85rem;
  color: var(--p-text-muted-color);
}
</style>
