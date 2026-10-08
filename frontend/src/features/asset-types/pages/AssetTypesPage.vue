<script setup lang="ts">
import Button from 'primevue/button'
import Column from 'primevue/column'
import ContextMenu from 'primevue/contextmenu'
import DataTable, { type DataTableRowContextMenuEvent } from 'primevue/datatable'
import IconField from 'primevue/iconfield'
import InputIcon from 'primevue/inputicon'
import InputText from 'primevue/inputtext'
import Menu from 'primevue/menu'
import type { MenuItem } from 'primevue/menuitem'
import Tag from 'primevue/tag'
import Textarea from 'primevue/textarea'
import { computed, reactive, ref, shallowRef, watch } from 'vue'
import { useRouter } from 'vue-router'
import AddCard from '@/components/AddCard.vue'
import CardGrid from '@/components/CardGrid.vue'
import EmptyState from '@/components/EmptyState.vue'
import TableSkeleton from '@/components/TableSkeleton.vue'
import EntityCard from '@/components/EntityCard.vue'
import FormDialog from '@/components/FormDialog.vue'
import IconAction from '@/components/IconAction.vue'
import InlineCell from '@/components/InlineCell.vue'
import OverviewFields, { type FieldDef } from '@/components/OverviewFields.vue'
import QuickEditDrawer from '@/components/QuickEditDrawer.vue'
import PageHeader from '@/components/PageHeader.vue'
import SegmentedFilter, { type SegmentOption } from '@/components/SegmentedFilter.vue'
import { useListContext } from '@/features/assets/listContext'
import { typeListLocation } from '@/features/assets/listQuery'
import type { AssetType } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { mayClose } from '@/lib/confirm'
import { formatDate } from '@/lib/dates'
import { changesOf, clearTab, emptyDraft, isDirty } from '@/lib/detailDraft'
import { useDirty, useFormErrors } from '@/lib/forms'
import { openLocation } from '@/lib/navigation'
import { onRowClick, useRowMenu } from '@/lib/tableRows'
import { queryString, useUrlState } from '@/lib/urlState'
import { useAssetTypes, useCreateAssetType } from '../api'
import { codeFromName, codeMark } from '../code'
import { useTypeArchive, useTypeOverviewSave } from '../overviewSave'
import KindMeter from '../components/KindMeter.vue'

// Danh sách loại: thẻ (mặc định) hoặc bảng; đang dùng / đã archive; tìm theo tên, mã.
// Cả ba nằm trên URL của tab như các danh sách khác.
const session = useSession()
const router = useRouter()
const canManage = computed(() => session.can(Perm.TypeManage))
const { data: types, isFetching, isLoading } = useAssetTypes(true, true)

type Show = 'active' | 'archived'
type Layout = 'cards' | 'table'
const { state, update } = useUrlState(
  (q) => ({
    q: queryString(q.q) ?? '',
    show: (q.show === 'archived' ? 'archived' : 'active') as Show,
    layout: (q.layout === 'table' ? 'table' : 'cards') as Layout,
  }),
  (s) => ({
    q: s.q || undefined,
    show: s.show === 'archived' ? s.show : undefined,
    layout: s.layout === 'table' ? s.layout : undefined,
  }),
)
const show = computed({ get: () => state.value.show, set: (v: Show) => update({ show: v }) })
const layout = computed({ get: () => state.value.layout, set: (v: Layout) => update({ layout: v }) })

const all = computed(() => types.value ?? [])
const visible = computed(() => {
  const q = state.value.q.trim().toLowerCase()
  return all.value
    .filter((t) => (state.value.show === 'archived' ? !!t.archived_at : !t.archived_at))
    .filter((t) => !q || `${t.name} ${t.code}`.toLowerCase().includes(q))
})

const showOptions = computed<SegmentOption<Show>[]>(() => [
  { label: 'Active', value: 'active', count: all.value.filter((t) => !t.archived_at).length },
  { label: 'Archived', value: 'archived', count: all.value.filter((t) => t.archived_at).length },
])
const layoutOptions: SegmentOption<Layout>[] = [
  { label: 'Cards', value: 'cards', icon: 'pi pi-th-large' },
  { label: 'Table', value: 'table', icon: 'pi pi-list' },
]

// Ô tìm kiếm: đợi gõ xong rồi mới đổi URL
const search = ref(state.value.q)
let timer: ReturnType<typeof setTimeout> | undefined
watch(search, (q) => {
  clearTimeout(timer)
  timer = setTimeout(() => update({ q }), 250)
})

function attrLine(t: AssetType) {
  const labels = t.attribute_labels ?? []
  if (!labels.length) return 'No attributes'
  return labels.slice(0, 3).join(' · ') + (labels.length > 3 ? ` · +${labels.length - 3}` : '')
}

// Mở danh sách tài sản của loại, giữ bộ lọc đã nhớ (Ctrl/⌘ hay chuột giữa mở tab mới)
const listContext = useListContext()
const listOf = (t: AssetType) => typeListLocation(t.id, listContext.views)
const settingsPath = (t: AssetType) => `/types/${t.id}/settings`
function openType(t: AssetType, e?: MouseEvent, newTab?: boolean) {
  openLocation(router, listOf(t), e, newTab)
}
const rowClick = onRowClick(openType)
const menu = ref<InstanceType<typeof ContextMenu>>()
// Cùng một danh sách hành động cho menu chuột phải (dòng, thẻ) và nút menu trên thẻ
const typeMenu = (t: AssetType): MenuItem[] => [
  { label: 'Open assets', icon: 'pi pi-arrow-right', command: () => openType(t) },
  { label: 'Open in new tab', icon: 'pi pi-external-link', command: () => openType(t, undefined, true) },
  { separator: true },
  { label: 'Type settings', icon: 'pi pi-cog', command: () => openLocation(router, settingsPath(t)) },
  { label: 'Quick edit', icon: 'pi pi-pencil', visible: canManage.value, command: () => openQuick(t) },
  { label: t.archived_at ? 'Restore' : 'Archive', icon: t.archived_at ? 'pi pi-replay' : 'pi pi-inbox', visible: canManage.value && !t.is_system, command: () => archiveType(t) },
]
const { items: menuItems, show: showMenu, clear: clearMenu } = useRowMenu<AssetType>(menu, typeMenu)
function onCardMenu(t: AssetType, e: MouseEvent) {
  e.preventDefault()
  showMenu({ originalEvent: e, data: t, index: 0 } as DataTableRowContextMenuEvent)
}
// Nút menu (☰) trên đầu thẻ: thay cho hàng nút hành động để thẻ gọn. Không xoá thẻ đang
// chọn khi menu đóng: Menu báo đóng sau hiệu ứng, lúc đó có thể đã mở cho thẻ khác.
const cardMenu = ref<InstanceType<typeof Menu>>()
const cardMenuType = shallowRef<AssetType | null>(null)
const cardMenuItems = computed(() => (cardMenuType.value ? typeMenu(cardMenuType.value) : []))
function toggleCardMenu(t: AssetType, e: MouseEvent) {
  cardMenuType.value = t
  cardMenu.value?.toggle(e)
}

// Loại mới: mã tự điền theo tên cho đến khi người dùng tự sửa mã
const creating = ref(false)
const code = ref('')
const codeTouched = ref(false)
const name = ref('')
const description = ref('')
const errors = useFormErrors()
const create = useCreateAssetType()
const form = useDirty(() => ({ c: code.value.trim(), n: name.value.trim(), d: description.value.trim() }))
watch(name, (n) => {
  if (!codeTouched.value) code.value = codeFromName(n)
})

function openCreate() {
  code.value = name.value = description.value = ''
  codeTouched.value = false
  errors.clear()
  form.reset()
  creating.value = true
}

async function submit() {
  errors.clear()
  try {
    const t = await create.mutateAsync({ code: code.value, name: name.value, description: description.value })
    creating.value = false
    // mở trang của loại ở Attributes, nút "Add attribute" sẵn focus
    await router.push({ path: settingsPath(t), query: { tab: 'attributes', new: '1' } })
  } catch (err) {
    errors.set(err)
  }
}

// Sửa nhanh từ danh sách: nhấp đúp tên để đổi tại chỗ; bút chì mở ngăn kéo (tên, mô tả)
const saveType = useTypeOverviewSave()
const archiveType = useTypeArchive()
const rename = (t: AssetType, name: string) => saveType(t, { name })
const quick = ref<AssetType | null>(null)
const quickDraft = reactive(emptyDraft())
const quickSaving = ref(false)
const quickFields: FieldDef[] = [
  { key: 'name', label: 'Name', maxlength: 100 },
  { key: 'code', label: 'Code', lock: 'Part of every asset tag, so it can’t change.' },
  { key: 'description', label: 'Description', kind: 'textarea' },
]
const quickSaved = computed(() => ({ name: quick.value?.name ?? '', code: quick.value?.code ?? '', description: quick.value?.description ?? '' }))
const quickOpen = computed({
  get: () => quick.value !== null,
  set: (v) => {
    if (!v) closeQuick()
  },
})
function closeQuick() {
  quick.value = null
  clearTab(quickDraft, 'overview')
}
function openQuick(t: AssetType) {
  clearTab(quickDraft, 'overview')
  quick.value = t
}
const quickIndex = computed(() => (quick.value ? visible.value.findIndex((t) => t.id === quick.value!.id) : -1))
async function moveQuick(step: number) {
  const next = visible.value[quickIndex.value + step]
  if (!next || !(await mayClose(isDirty(quickDraft)))) return
  openQuick(next)
}
async function saveQuick() {
  const t = quick.value
  if (!t) return
  quickSaving.value = true
  try {
    if (await saveType(t, changesOf(quickDraft, 'overview'))) clearTab(quickDraft, 'overview')
  } finally {
    quickSaving.value = false
  }
}
// danh sách nạp lại sau khi lưu: ngăn kéo theo bản mới (version mới)
watch(types, (list) => {
  if (quick.value) quick.value = list?.find((t) => t.id === quick.value!.id) ?? null
})
</script>

<template>
  <section>
    <PageHeader title="Asset types" subtitle="What kinds of things you track, and which fields each one has.">
      <Button v-if="canManage" label="New type" icon="pi pi-plus" @click="openCreate" />
    </PageHeader>

    <div class="toolbar">
      <SegmentedFilter v-model="show" :options="showOptions" label="Show" />
      <IconField>
        <InputIcon class="pi pi-search" />
        <InputText v-model="search" placeholder="Search name or code" aria-label="Search asset types" />
      </IconField>
      <SegmentedFilter v-model="layout" :options="layoutOptions" label="Layout" icon-only class="end" />
    </div>
    <KindMeter v-if="state.show === 'active'" legend class="legend-row" />

    <ContextMenu ref="menu" :model="menuItems" @hide="clearMenu" />
    <Menu ref="cardMenu" :model="cardMenuItems" popup />

    <CardGrid v-if="state.layout === 'cards'">
      <EntityCard
        v-for="t in visible"
        :key="t.id"
        :to="listOf(t)"
        :label="`${t.name} assets`"
        :dimmed="!!t.archived_at"
        @menu="(e) => onCardMenu(t, e)"
      >
        <header class="card-head">
          <span class="mark">{{ codeMark(t.code) }}</span>
          <div class="title">
            <h3>
              <InlineCell :value="t.name" label="name" :editable="canManage" @save="(v) => rename(t, v)">{{ t.name }}</InlineCell>
            </h3>
            <code>{{ t.code }}</code>
          </div>
          <span v-if="t.is_system" v-tooltip.top="'Built-in type'" class="lock" aria-label="Built-in">
            <i class="pi pi-lock" />
          </span>
          <IconAction icon="pi pi-bars" :label="`Actions for ${t.name}`" aria-haspopup="menu" @click="(e) => toggleCardMenu(t, e)" />
        </header>
        <p class="desc">{{ t.description || 'No description.' }}</p>
        <Tag
          v-if="t.archived_at"
          :value="`Archived ${formatDate(t.archived_at)}`"
          severity="secondary"
          icon="pi pi-inbox"
          class="archived-tag"
        />
        <KindMeter v-else :type="t" />
        <dl class="stats">
          <div>
            <dt>Assets</dt>
            <dd>{{ t.asset_count ?? 0 }}</dd>
          </div>
          <div>
            <dt>Attributes</dt>
            <dd>{{ t.attribute_count ?? 0 }}</dd>
          </div>
        </dl>
        <p class="attrs">{{ attrLine(t) }}</p>
      </EntityCard>
      <AddCard v-if="canManage && state.show === 'active' && !state.q" label="New type" @click="openCreate" />
      <p v-if="!visible.length && (state.q || state.show === 'archived')" class="empty">
        {{ state.q ? `No types match "${state.q}".` : 'No archived types.' }}
      </p>
    </CardGrid>

    <DataTable
      v-else
      :value="visible"
      :loading="isFetching"
      data-key="id"
      removable-sort
      row-hover
      :row-class="() => 'clickable-row'"
      @row-click="rowClick"
      @row-contextmenu="showMenu"
    >
      <Column header="Name" sort-field="name" sortable>
        <template #body="{ data: t }: { data: AssetType }">
          <div class="name-cell">
            <span class="mark small">{{ codeMark(t.code) }}</span>
            <div>
              <InlineCell :value="t.name" label="name" :editable="canManage" @save="(v) => rename(t, v)">
                <RouterLink :to="listOf(t)">{{ t.name }}</RouterLink>
              </InlineCell>
              <code class="sub-code">{{ t.code }}</code>
            </div>
          </div>
        </template>
      </Column>
      <Column field="description" header="Description" sortable>
        <template #body="{ data: t }: { data: AssetType }">
          <span class="clip-text" :title="t.description || undefined">{{ t.description }}</span>
        </template>
      </Column>
      <Column header="Attributes" sort-field="attribute_count" sortable>
        <template #body="{ data: t }: { data: AssetType }">
          {{ t.attribute_count ?? 0 }}
          <span class="cell-sub">{{ (t.attribute_labels ?? []).slice(0, 2).join(', ') }}</span>
        </template>
      </Column>
      <Column header="Assets" sort-field="asset_count" sortable body-class="num">
        <template #body="{ data: t }: { data: AssetType }">{{ t.asset_count ?? 0 }}</template>
      </Column>
      <Column header="In service" header-style="min-width: 9rem">
        <template #body="{ data: t }: { data: AssetType }">
          <Tag v-if="t.archived_at" value="Archived" severity="secondary" />
          <KindMeter v-else :type="t" />
        </template>
      </Column>
      <Column header="" header-style="width: 8rem">
        <template #body="{ data: t }: { data: AssetType }">
          <div class="row-actions">
            <IconAction v-if="canManage" icon="pi pi-pencil" label="Quick edit" @click="openQuick(t)" />
            <IconAction
              v-if="canManage && !t.is_system"
              :icon="t.archived_at ? 'pi pi-replay' : 'pi pi-inbox'"
              :label="t.archived_at ? 'Restore' : 'Archive'"
              @click="archiveType(t)"
            />
            <IconAction icon="pi pi-cog" label="Type settings" :to="settingsPath(t)" />
          </div>
        </template>
      </Column>
      <template #empty>
        <TableSkeleton v-if="isLoading" />
        <EmptyState v-else icon="pi pi-sitemap" :text="state.q ? `No types match &quot;${state.q}&quot;.` : 'No asset types here.'" />
      </template>
    </DataTable>

    <QuickEditDrawer
      v-if="quick"
      v-model:visible="quickOpen"
      :title="quick.name"
      icon="sitemap"
      :dirty="isDirty(quickDraft)"
      :busy="quickSaving"
      :can-prev="quickIndex > 0"
      :can-next="quickIndex >= 0 && quickIndex < visible.length - 1"
      @save="saveQuick"
      @prev="moveQuick(-1)"
      @next="moveQuick(1)"
      @open-page="router.push(settingsPath(quick))"
    >
      <OverviewFields :fields="quickFields" :saved="quickSaved" :draft="quickDraft" stacked />
    </QuickEditDrawer>

    <FormDialog
      v-model:visible="creating"
      icon="sitemap"
      title="New asset type"
      action="Create and add attributes"
      :busy="create.isPending.value"
      :error="errors.general.value"
      :dirty="form.dirty.value"
      @submit="submit"
    >
      <div class="field">
        <label for="type-name">Name</label>
        <InputText id="type-name" v-model="name" required placeholder="Network gear" autofocus />
        <small v-if="errors.fields.value.name" class="field-error">{{ errors.fields.value.name }}</small>
      </div>
      <div class="field">
        <label for="type-code">Code</label>
        <InputText id="type-code" v-model="code" required class="mono" placeholder="NETWORK_GEAR" @input="codeTouched = true" />
        <small>Filled from the name. A-Z, 0-9, _ or -. It can't change later.</small>
        <small v-if="errors.fields.value.code" class="field-error">{{ errors.fields.value.code }}</small>
      </div>
      <div class="field">
        <label for="type-desc">Description</label>
        <Textarea id="type-desc" v-model="description" rows="3" />
      </div>
    </FormDialog>
  </section>
</template>

<style scoped>
.end {
  margin-left: auto;
}
.legend-row {
  margin: -0.25rem 0 0.75rem;
}
.card-head {
  display: flex;
  align-items: center;
  gap: 0.65rem;
}
.title {
  flex: 1;
  min-width: 0;
}
.title h3 {
  font-size: 1rem;
}
.title code,
.sub-code {
  display: block;
  font-size: 0.78rem;
  color: var(--p-text-muted-color);
}
.mark {
  flex: none;
  display: grid;
  place-items: center;
  width: 2.4rem;
  height: 2.4rem;
  border-radius: 8px;
  background: var(--app-soft);
  font: 700 0.8rem var(--app-mono);
  color: var(--p-text-color);
}
.mark.small {
  width: 2rem;
  height: 2rem;
  font-size: 0.7rem;
}
/* ổ khoá chiếm ô vuông bằng nút cài đặt bên cạnh để hai cái thẳng hàng */
.lock {
  flex: none;
  display: inline-grid;
  place-items: center;
  width: 2rem;
  height: 2rem;
  color: var(--p-text-muted-color);
  font-size: 0.8rem;
}
.desc {
  margin: 0;
  min-height: 2.6em;
  color: var(--p-text-muted-color);
  font-size: 0.875rem;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.archived-tag {
  align-self: flex-start;
}
.stats {
  display: flex;
  gap: 1.5rem;
  margin: 0;
}
.stats div {
  display: flex;
  flex-direction: column;
}
.stats dt {
  font-size: 0.75rem;
  color: var(--p-text-muted-color);
}
.stats dd {
  margin: 0;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.attrs {
  margin: 0;
  font-size: 0.82rem;
  color: var(--p-text-muted-color);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.empty {
  grid-column: 1 / -1;
  padding: 2rem 1rem;
  border: 1px dashed var(--app-line);
  border-radius: 10px;
  text-align: center;
  color: var(--p-text-muted-color);
}
.name-cell {
  display: flex;
  align-items: center;
  gap: 0.6rem;
}
.cell-sub {
  display: block;
  font-size: 0.78rem;
  color: var(--p-text-muted-color);
}
:deep(td.num) {
  font-variant-numeric: tabular-nums;
}
.mono {
  font-family: var(--app-mono);
}
</style>
