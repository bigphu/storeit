<script setup lang="ts">
import Button from 'primevue/button'
import Card from 'primevue/card'
import Column from 'primevue/column'
import ContextMenu from 'primevue/contextmenu'
import DataTable, { type DataTableRowContextMenuEvent } from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import IconField from 'primevue/iconfield'
import InputIcon from 'primevue/inputicon'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import MeterGroup from 'primevue/metergroup'
import SelectButton from 'primevue/selectbutton'
import Tag from 'primevue/tag'
import Textarea from 'primevue/textarea'
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useListContext } from '@/features/assets/listContext'
import { typeListLocation } from '@/features/assets/listQuery'
import type { AssetType } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { formatDate } from '@/lib/dates'
import { useFormErrors } from '@/lib/forms'
import { openLocation, wantsNewTab } from '@/lib/navigation'
import { isRowControl, onRowClick, useRowMenu } from '@/lib/tableRows'
import { queryString, useUrlState } from '@/lib/urlState'
import { useAssetTypes, useCreateAssetType } from '../api'
import { codeFromName, codeMark } from '../code'

// Danh sách loại: thẻ (mặc định) hoặc bảng; đang dùng / đã archive; tìm theo tên, mã.
// Cả ba nằm trên URL của tab như các danh sách khác.
const session = useSession()
const router = useRouter()
const canManage = computed(() => session.can(Perm.TypeManage))
const { data: types, isFetching } = useAssetTypes(true, true)

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

const all = computed(() => types.value ?? [])
const counts = computed(() => ({
  active: all.value.filter((t) => !t.archived_at).length,
  archived: all.value.filter((t) => t.archived_at).length,
}))
const visible = computed(() => {
  const q = state.value.q.trim().toLowerCase()
  return all.value
    .filter((t) => (state.value.show === 'archived' ? !!t.archived_at : !t.archived_at))
    .filter((t) => !q || `${t.name} ${t.code}`.toLowerCase().includes(q))
})

const showOptions = computed(() => [
  { label: 'Active', value: 'active', count: counts.value.active },
  { label: 'Archived', value: 'archived', count: counts.value.archived },
])
const layoutOptions = [
  { icon: 'pi pi-th-large', value: 'cards', label: 'Cards' },
  { icon: 'pi pi-list', value: 'table', label: 'Table' },
]

// Ô tìm kiếm: đợi gõ xong rồi mới đổi URL
const search = ref(state.value.q)
let timer: ReturnType<typeof setTimeout> | undefined
watch(search, (q) => {
  clearTimeout(timer)
  timer = setTimeout(() => update({ q }), 250)
})

// Thanh theo kind: xanh có thể giao, xanh dương đang dùng, cam không dùng được
const KIND_COLORS = {
  available: 'var(--p-green-500)',
  in_use: 'var(--p-sky-500)',
  unavailable: 'var(--p-orange-500)',
}
function meter(t: AssetType) {
  const k = t.kind_counts
  if (!k) return []
  return [
    { label: 'Available', value: k.available, color: KIND_COLORS.available },
    { label: 'In use', value: k.in_use, color: KIND_COLORS.in_use },
    { label: 'Unavailable', value: k.unavailable, color: KIND_COLORS.unavailable },
  ]
}
const meterTitle = (t: AssetType) =>
  t.kind_counts ? `${t.kind_counts.available} available · ${t.kind_counts.in_use} in use · ${t.kind_counts.unavailable} unavailable` : ''
function attrLine(t: AssetType) {
  const labels = t.attribute_labels ?? []
  if (!labels.length) return 'No attributes'
  return labels.slice(0, 3).join(' · ') + (labels.length > 3 ? ` · +${labels.length - 3}` : '')
}

// Mở danh sách tài sản của loại, giữ bộ lọc đã nhớ (Ctrl/⌘ hay chuột giữa mở tab mới)
const listContext = useListContext()
function openType(t: AssetType, e?: MouseEvent, newTab?: boolean) {
  openLocation(router, typeListLocation(t.id, listContext.views), e, newTab)
}
const settingsPath = (t: AssetType) => `/types/${t.id}/settings`
function onCardClick(t: AssetType, e: MouseEvent) {
  if (isRowControl(e.target)) return
  openType(t, e)
}
function onCardAux(t: AssetType, e: MouseEvent) {
  if (wantsNewTab(e) && !isRowControl(e.target)) openType(t, e)
}
const rowClick = onRowClick(openType)
const menu = ref<InstanceType<typeof ContextMenu>>()
const { items: menuItems, show: showMenu, clear: clearMenu } = useRowMenu<AssetType>(menu, (t) => [
  { label: 'Open assets', icon: 'pi pi-arrow-right', command: () => openType(t) },
  { label: 'Open in new tab', icon: 'pi pi-external-link', command: () => openType(t, undefined, true) },
  { separator: true },
  { label: 'Type settings', icon: 'pi pi-cog', command: () => openLocation(router, settingsPath(t)) },
])
function onCardMenu(t: AssetType, e: MouseEvent) {
  showMenu({ originalEvent: e, data: t, index: 0 } as DataTableRowContextMenuEvent)
}

// Loại mới: mã tự điền theo tên cho đến khi người dùng tự sửa mã
const creating = ref(false)
const code = ref('')
const codeTouched = ref(false)
const name = ref('')
const description = ref('')
const errors = useFormErrors()
const create = useCreateAssetType()
watch(name, (n) => {
  if (!codeTouched.value) code.value = codeFromName(n)
})

function openCreate() {
  code.value = name.value = description.value = ''
  codeTouched.value = false
  errors.clear()
  creating.value = true
}

async function submit() {
  errors.clear()
  try {
    const t = await create.mutateAsync({ code: code.value, name: name.value, description: description.value })
    creating.value = false
    // thêm thuộc tính ở trang cài đặt
    await router.push(settingsPath(t))
  } catch (err) {
    errors.set(err)
  }
}
</script>

<template>
  <section>
    <div class="page-header">
      <div>
        <h1>Asset types</h1>
        <p class="sub">What kinds of things you track, and which fields each one has.</p>
      </div>
      <Button v-if="canManage" label="New type" icon="pi pi-plus" @click="openCreate" />
    </div>

    <div class="toolbar">
      <SelectButton
        :model-value="state.show"
        :options="showOptions"
        option-value="value"
        :allow-empty="false"
        aria-label="Show"
        @update:model-value="(v: Show) => update({ show: v })"
      >
        <template #option="{ option }">
          {{ option.label }} <span class="seg-count">{{ option.count }}</span>
        </template>
      </SelectButton>
      <IconField>
        <InputIcon class="pi pi-search" />
        <InputText v-model="search" placeholder="Search name or code" aria-label="Search asset types" />
      </IconField>
      <SelectButton
        :model-value="state.layout"
        :options="layoutOptions"
        option-value="value"
        :allow-empty="false"
        aria-label="Layout"
        class="end"
        @update:model-value="(v: Layout) => update({ layout: v })"
      >
        <template #option="{ option }">
          <i v-tooltip.top="option.label" :class="option.icon" :aria-label="option.label" />
        </template>
      </SelectButton>
    </div>
    <div v-if="state.show === 'active'" class="legend" aria-hidden="true">
      <span><i :style="{ background: KIND_COLORS.available }" />Available</span>
      <span><i :style="{ background: KIND_COLORS.in_use }" />In use</span>
      <span><i :style="{ background: KIND_COLORS.unavailable }" />Unavailable</span>
    </div>

    <ContextMenu ref="menu" :model="menuItems" @hide="clearMenu" />

    <div v-if="state.layout === 'cards'" class="type-grid">
      <Card
        v-for="t in visible"
        :key="t.id"
        class="type-card"
        :class="{ archived: t.archived_at }"
        tabindex="0"
        role="link"
        :aria-label="`${t.name} assets`"
        @click="(e: MouseEvent) => onCardClick(t, e)"
        @auxclick="(e: MouseEvent) => onCardAux(t, e)"
        @mousedown.middle.prevent
        @keydown.enter.self="openType(t)"
        @contextmenu.prevent="(e: MouseEvent) => onCardMenu(t, e)"
      >
        <template #content>
          <div class="card-body">
            <header>
              <span class="mark">{{ codeMark(t.code) }}</span>
              <div class="title">
                <h3>{{ t.name }}</h3>
                <code>{{ t.code }}</code>
              </div>
              <i v-if="t.is_system" v-tooltip.top="'Built-in type'" class="pi pi-lock lock" aria-label="Built-in" />
              <Button v-slot="slot" v-tooltip.top="'Type settings'" icon="pi pi-cog" text rounded size="small" as-child>
                <RouterLink :to="settingsPath(t)" :class="slot.class" :aria-label="`${t.name} settings`">
                  <i class="pi pi-cog" />
                </RouterLink>
              </Button>
            </header>
            <p class="desc">{{ t.description || 'No description.' }}</p>
            <Tag v-if="t.archived_at" :value="`Archived ${formatDate(t.archived_at)}`" severity="secondary" icon="pi pi-inbox" class="archived-tag" />
            <div v-else :title="meterTitle(t)">
              <MeterGroup :value="meter(t)" :max="Math.max(t.asset_count ?? 0, 1)" class="kind-meter">
                <template #label><span /></template>
              </MeterGroup>
            </div>
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
          </div>
        </template>
      </Card>
      <button v-if="canManage && state.show === 'active' && !state.q" type="button" class="new-card" @click="openCreate">
        <i class="pi pi-plus" />
        <span>New type</span>
      </button>
      <p v-if="!visible.length && (state.q || state.show === 'archived')" class="empty">
        {{ state.q ? `No types match "${state.q}".` : 'No archived types.' }}
      </p>
    </div>

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
              <RouterLink :to="typeListLocation(t.id, listContext.views)">{{ t.name }}</RouterLink>
              <code class="sub-code">{{ t.code }}</code>
            </div>
          </div>
        </template>
      </Column>
      <Column field="description" header="Description" sortable />
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
          <div v-else :title="meterTitle(t)">
            <MeterGroup :value="meter(t)" :max="Math.max(t.asset_count ?? 0, 1)" class="kind-meter">
              <template #label><span /></template>
            </MeterGroup>
          </div>
        </template>
      </Column>
      <Column header="" header-style="width: 4rem">
        <template #body="{ data: t }: { data: AssetType }">
          <div class="row-actions">
            <Button v-slot="slot" v-tooltip.top="'Type settings'" icon="pi pi-cog" text rounded size="small" as-child>
              <RouterLink :to="settingsPath(t)" :class="slot.class" aria-label="Type settings">
                <i class="pi pi-cog" />
              </RouterLink>
            </Button>
          </div>
        </template>
      </Column>
      <template #empty>{{ state.q ? `No types match "${state.q}".` : 'No asset types here.' }}</template>
    </DataTable>

    <Dialog v-model:visible="creating" modal header="New asset type" :style="{ width: '32rem' }">
      <form class="form" @submit.prevent="submit">
        <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>
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
        <div class="actions">
          <Button type="submit" label="Create and add attributes" :loading="create.isPending.value" />
          <Button label="Cancel" severity="secondary" text @click="creating = false" />
        </div>
      </form>
    </Dialog>
  </section>
</template>

<style scoped>
.sub {
  margin: 0.2rem 0 0;
  color: var(--p-text-muted-color);
}
.end {
  margin-left: auto;
}
.seg-count {
  font: 0.75rem var(--app-mono);
  color: var(--p-text-muted-color);
}
.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem 0.9rem;
  margin: -0.25rem 0 0.75rem;
  font-size: 0.78rem;
  color: var(--p-text-muted-color);
}
.legend i {
  display: inline-block;
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 50%;
  margin-right: 0.3rem;
}
.type-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(15.5rem, 1fr));
  gap: 0.9rem;
}
.type-card {
  cursor: pointer;
  border: 1px solid var(--app-line);
  box-shadow: none;
  transition: box-shadow 0.15s ease;
}
.type-card:hover,
.type-card:focus-visible {
  box-shadow: 0 0 0 1px var(--app-accent);
}
.type-card.archived {
  opacity: 0.8;
}
.type-card :deep(.p-card-body) {
  padding: 0.95rem;
}
.card-body {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
}
.card-body header {
  display: flex;
  align-items: flex-start;
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
/* ổ khoá chiếm ô vuông bằng nút cài đặt (nút nhỏ chỉ có icon) để hai cái thẳng hàng */
.lock {
  flex: none;
  display: inline-grid;
  place-items: center;
  width: var(--p-button-sm-icon-only-width, 2rem);
  height: var(--p-button-sm-icon-only-width, 2rem);
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
.kind-meter :deep(.p-metergroup-label-list) {
  display: none;
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
.new-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  min-height: 12rem;
  border: 1px dashed var(--app-line);
  border-radius: var(--p-card-border-radius, 12px);
  background: transparent;
  color: var(--p-text-muted-color);
  font: inherit;
  cursor: pointer;
}
.new-card:hover {
  background: var(--p-datatable-row-hover-background, var(--app-soft));
  color: var(--p-text-color);
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
