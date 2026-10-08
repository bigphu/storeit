<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Column from 'primevue/column'
import DataTable, { type DataTableRowReorderEvent } from 'primevue/datatable'
import InputText from 'primevue/inputtext'
import Tag from 'primevue/tag'
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import EmptyState from '@/components/EmptyState.vue'
import InlineCell from '@/components/InlineCell.vue'
import OverviewFields, { type FieldDef } from '@/components/OverviewFields.vue'
import QuickEditDrawer from '@/components/QuickEditDrawer.vue'
import TableSkeleton from '@/components/TableSkeleton.vue'
import RowMenuButton from '@/components/RowMenuButton.vue'
import RowMenus from '@/components/RowMenus.vue'
import PageHeader from '@/components/PageHeader.vue'
import type { Status, StatusKind } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { runAction } from '@/lib/actions'
import { mayClose } from '@/lib/confirm'
import { useQuickDrawer } from '@/lib/quickDrawer'
import { changesOf, clearTab, emptyDraft, isDirty } from '@/lib/detailDraft'
import { useFormErrors } from '@/lib/forms'
import { openLocation } from '@/lib/navigation'
import { notify } from '@/lib/notify'
import { useListTable, useRowMenu } from '@/lib/tableRows'
import { kindSeverity, useCreateStatus, useReorderStatuses, useStatuses } from '../api'
import { archiveBlock, KIND_INFO, KIND_ORDER, lanes, moveBy, orderAfterMove } from '../lanes'
import { useStatusLifecycle, useStatusOverviewSave } from '../overviewSave'

// Status chia theo kind thành bốn làn; kéo thả (hay Alt+↑/↓ trên tên) đổi thứ tự trong làn.
// Bấm dòng mở trang của status; bút chì cạnh tên để đổi tại chỗ; nút thanh trượt (pi-sliders-h) mở ngăn kéo sửa nhanh
const session = useSession()
const router = useRouter()
const canManage = computed(() => session.can(Perm.StatusManage))
const showArchived = ref(false)
const { data: statuses, isLoading } = useStatuses(showArchived, true)

const byKind = computed(() => lanes(statuses.value ?? []))
const activeOf = (k: StatusKind) => byKind.value[k].filter((s) => !s.archived_at)
const archivedOf = (k: StatusKind) => byKind.value[k].filter((s) => s.archived_at)
const laneCount = (k: StatusKind) => activeOf(k).reduce((n, s) => n + (s.asset_count ?? 0), 0)

// Đổi thứ tự: gửi thứ tự của mọi status đang dùng
const reorder = useReorderStatuses()
function saveLane(kind: StatusKind, laneIds: string[]) {
  const all = statuses.value ?? []
  const before = all
    .filter((s) => !s.archived_at)
    .sort((a, b) => a.position - b.position)
    .map((s) => s.id)
  const after = orderAfterMove(all, kind, laneIds)
  void runAction({
    run: () => reorder.mutateAsync(after),
    done: 'Statuses reordered.',
    failed: "Couldn't save the new order.",
    undo: () => reorder.mutateAsync(before),
    undone: 'Order put back.',
    undoFailed: "Couldn't put the order back.",
  })
}
function onReorder(kind: StatusKind, e: DataTableRowReorderEvent) {
  saveLane(kind, (e.value as Status[]).map((s) => s.id))
}
async function moveKey(s: Status, delta: number) {
  const ids = moveBy(activeOf(s.kind).map((x) => x.id), s.id, delta)
  if (!ids) return
  saveLane(s.kind, ids)
  // giữ focus trên tên vừa dời để bấm tiếp
  await nextTick()
  document.querySelector<HTMLElement>(`[data-status-name="${s.id}"]`)?.focus()
}

const lifecycle = useStatusLifecycle()
const makeDefault = (s: Status) => lifecycle.makeDefault(s, statuses.value ?? [])
const archiveStatus = (s: Status) => lifecycle.archive(s)
const restoreStatus = (s: Status) => lifecycle.restore(s)

// Thêm status ở cuối làn; kind lấy từ làn
const create = useCreateStatus()
const adding = ref<StatusKind | null>(null)
const newName = ref('')
const addErrors = useFormErrors()
async function startAdd(k: StatusKind) {
  adding.value = k
  newName.value = ''
  addErrors.clear()
  await nextTick()
  document.getElementById(`add-${k}`)?.focus()
}
async function submitAdd() {
  const kind = adding.value
  if (!kind || !newName.value.trim()) return
  addErrors.clear()
  const last = Math.max(0, ...(statuses.value ?? []).map((s) => s.position))
  try {
    const created = await create.mutateAsync({ name: newName.value, kind, position: last + 1 })
    notify.success(`${created.name} added to ${KIND_INFO[kind].label}.`, {
      undo: () => void lifecycle.archive(created),
      action: { label: 'Open', run: () => router.push(`/statuses/${created.id}`) },
    })
    newName.value = ''
    await nextTick()
    document.getElementById(`add-${kind}`)?.focus()
  } catch (err) {
    addErrors.set(err)
  }
}

// Sửa nhanh: bút chì cạnh tên để đổi tại chỗ; nút thanh trượt (pi-sliders-h) mở ngăn kéo (tên, kind khoá)
const saveStatus = useStatusOverviewSave()
const rename = (st: Status, name: string) => saveStatus(st, { name })
const openStatus = (st: Status, e?: MouseEvent) => openLocation(router, `/statuses/${st.id}`, e)
// mỗi làn một bảng: bấm dòng mở status, nút cuối dòng cho hàng đang dùng của làn đó
// menu của dòng (chuột phải và nút ☰): mở, sửa nhanh, archive (bị chặn thì nói lý do)
const rowMenu = useRowMenu<Status>((st) => {
  const block = archiveBlock(st)
  return [
    { label: 'Open', icon: 'pi pi-arrow-right', command: () => openStatus(st) },
    { label: 'Open in new tab', icon: 'pi pi-external-link', command: () => openLocation(router, `/statuses/${st.id}`, undefined, true) },
    ...(canManage.value
      ? [
          { separator: true },
          { label: 'Quick edit', icon: 'pi pi-sliders-h', command: () => openQuick(st) },
          { label: block ? `Archive (${block.toLowerCase()})` : 'Archive', icon: 'pi pi-inbox', disabled: !!block, command: () => archiveStatus(st) },
        ]
      : []),
  ]
})
const laneTables = Object.fromEntries(
  KIND_ORDER.map((k) => [k, useListTable<Status>({ open: (st, e) => openStatus(st, e), showMenu: rowMenu.showContext })]),
) as Record<
  (typeof KIND_ORDER)[number],
  ReturnType<typeof useListTable<Status>>
>
// thứ tự đi qua bằng ↑/↓ trong ngăn kéo: theo làn, rồi theo vị trí
const ordered = computed(() => KIND_ORDER.flatMap((k) => activeOf(k)))
const quick = ref<Status | null>(null)
const quickDraft = reactive(emptyDraft())
const quickSaving = ref(false)
const quickFields: FieldDef[] = [
  { key: 'name', label: 'Name', maxlength: 100 },
  { key: 'kind', label: 'Kind', lock: 'The kind decides how assets behave, so it can’t change.' },
]
const quickSaved = computed(() => ({ name: quick.value?.name ?? '', kind: quick.value ? KIND_INFO[quick.value.kind].label : '' }))
// mở/đóng ngăn kéo: mục giữ lại đến khi trượt ra xong rồi mới xoá cùng bản nháp
const quickDrawer = useQuickDrawer(quick, () => clearTab(quickDraft, 'overview'))
const quickOpen = quickDrawer.visible
function openQuick(st: Status) {
  quickDrawer.open(st)
}
const quickIndex = computed(() => (quick.value ? ordered.value.findIndex((x) => x.id === quick.value!.id) : -1))
async function moveQuick(step: number) {
  const next = ordered.value[quickIndex.value + step]
  if (!next || !(await mayClose(isDirty(quickDraft)))) return
  openQuick(next)
}
async function saveQuick() {
  const st = quick.value
  if (!st) return
  quickSaving.value = true
  try {
    if (await saveStatus(st, changesOf(quickDraft, 'overview'))) clearTab(quickDraft, 'overview')
  } finally {
    quickSaving.value = false
  }
}
watch(statuses, (list) => {
  if (quick.value) quick.value = list?.find((x) => x.id === quick.value!.id) ?? null
})
</script>

<template>
  <section>
    <PageHeader title="Statuses" subtitle="Every asset has one status. Its kind decides how the asset behaves." />
    <div class="toolbar">
      <Checkbox v-model="showArchived" input-id="show-archived" binary />
      <label for="show-archived">Show archived</label>
      <span v-if="canManage" class="hint end">Drag to reorder within a kind · Alt+↑/↓ on a name</span>
    </div>

    <div class="lanes">
      <RowMenus :menu="rowMenu" />
      <section v-for="k in KIND_ORDER" :key="k" class="lane" :aria-label="KIND_INFO[k].label">
        <header>
          <div class="lane-top">
            <Tag :value="KIND_INFO[k].label" :severity="kindSeverity(k)" />
            <span class="count">{{ laneCount(k) }} assets</span>
          </div>
          <p>{{ KIND_INFO[k].text }}</p>
        </header>

        <DataTable
          :value="activeOf(k)"
          data-key="id"
          :show-headers="false"
          size="small"
          class="lane-table"
          table-style="width: 100%; table-layout: fixed"
          scrollable
          v-bind="laneTables[k].bind"
          @row-reorder="(e: DataTableRowReorderEvent) => onReorder(k, e)"
        >
          <Column v-if="canManage" row-reorder row-reorder-icon="pi pi-grip-vertical" header-style="width: 2.5rem" body-style="width: 2.5rem" />
          <Column>
            <template #body="{ data: s }: { data: Status }">
              <div class="name-cell">
                <InlineCell
                  class="status-name"
                  :value="s.name"
                  label="name"
                  :editable="canManage"
                  :tabindex="canManage ? 0 : undefined"
                  :data-status-name="s.id"
                  @save="(v) => rename(s, v)"
                 
                  @keydown.alt.up.prevent="moveKey(s, -1)"
                  @keydown.alt.down.prevent="moveKey(s, 1)"
                >
                  {{ s.name }}
                </InlineCell>
                <span v-if="s.is_default" class="default-pill"><i class="pi pi-star-fill" />Default</span>
                <Button
                  v-else-if="canManage && k !== 'retired'"
                  label="Make default"
                  icon="pi pi-star"
                  text
                  size="small"
                  class="make-default"
                  @click="makeDefault(s)"
                />
                <i v-if="s.is_system" v-tooltip.top="'Built-in'" class="pi pi-lock lock" aria-label="Built-in" />
              </div>
            </template>
          </Column>
          <Column header-style="width: 3.5rem" body-style="width: 3.5rem" body-class="num-cell">
            <template #body="{ data: s }: { data: Status }">{{ s.asset_count ?? '' }}</template>
          </Column>
          <!-- nút ☰ ở mép phải của làn -->
          <Column frozen align-frozen="right" header-style="width: 3rem" body-style="width: 3rem" header-class="row-menu-col" body-class="row-menu-col">
            <template #body="{ data: s, index }: { data: Status; index: number }">
              <RowMenuButton :active="laneTables[k].active.isActive(index)" @open="(e) => rowMenu.toggle(s, e)" />
            </template>
          </Column>
          <template #empty>
            <TableSkeleton v-if="isLoading" />
            <EmptyState v-else icon="pi pi-tag" :text="`No ${KIND_INFO[k].label.toLowerCase()} statuses.`" />
          </template>
        </DataTable>

        <ul v-if="showArchived && archivedOf(k).length" class="archived-list">
          <li v-for="s in archivedOf(k)" :key="s.id">
            <span class="archived-name">{{ s.name }}</span>
            <Tag value="Archived" severity="secondary" />
            <span class="count">{{ s.asset_count ?? '' }}</span>
            <Button v-if="canManage" label="Restore" icon="pi pi-replay" text size="small" @click="restoreStatus(s)" />
          </li>
        </ul>

        <template v-if="canManage">
          <form v-if="adding === k" class="add-row" @submit.prevent="submitAdd">
            <InputText
              :id="`add-${k}`"
              v-model="newName"
              size="small"
              :placeholder="`New ${KIND_INFO[k].label.toLowerCase()} status`"
              aria-label="New status name"
              maxlength="100"
              @keydown.esc="adding = null"
            />
            <Button type="submit" label="Add" size="small" :loading="create.isPending.value" :disabled="!newName.trim()" />
            <Button label="Cancel" size="small" text severity="secondary" @click="adding = null" />
            <small v-if="addErrors.general.value || addErrors.fields.value.name" class="field-error">
              {{ addErrors.fields.value.name ?? addErrors.general.value }}
            </small>
          </form>
          <Button v-else label="Add status" icon="pi pi-plus" text size="small" class="add-btn" @click="startAdd(k)" />
        </template>
      </section>
    </div>

    <QuickEditDrawer
      v-if="quick"
      v-model:visible="quickOpen"
      @closed="quickDrawer.closed()"
      :title="quick.name"
      icon="tag"
      :dirty="isDirty(quickDraft)"
      :busy="quickSaving"
      :can-prev="quickIndex > 0"
      :can-next="quickIndex >= 0 && quickIndex < ordered.length - 1"
      @save="saveQuick"
      @prev="moveQuick(-1)"
      @next="moveQuick(1)"
      @open-page="router.push(`/statuses/${quick.id}`)"
    >
      <OverviewFields :fields="quickFields" :saved="quickSaved" :draft="quickDraft" stacked />
    </QuickEditDrawer>
  </section>
</template>

<style scoped>
.hint {
  color: var(--p-text-muted-color);
  font-size: 0.85rem;
}
.end {
  margin-left: auto;
}
.lanes {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem;
}
@media (max-width: 900px) {
  .lanes {
    grid-template-columns: minmax(0, 1fr);
  }
}
.lane {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  min-width: 0;
  /* làn là <section>: bỏ margin "section + section" chung của base.css */
  margin: 0;
  padding: 0.9rem;
  border: 1px solid var(--app-line);
  border-radius: 10px;
  background: var(--p-content-background);
}
.lane header p {
  margin: 0.35rem 0 0;
  color: var(--p-text-muted-color);
  font-size: 0.85rem;
}
.lane-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}
.count {
  font: 0.78rem var(--app-mono);
  color: var(--p-text-muted-color);
  font-variant-numeric: tabular-nums;
}
/* bảng trong làn: không khung, không tiêu đề */
.lane-table {
  border: 0;
  border-radius: 0;
}
.lane-table :deep(.p-datatable-tbody > tr > td) {
  border-bottom: 0;
}
.lane-table :deep(.p-datatable-tbody > tr > td.num-cell) {
  text-align: right;
  font: 0.78rem var(--app-mono);
  color: var(--p-text-muted-color);
}
.name-cell {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-width: 0;
}
.status-name {
  min-width: 0;
  border: 0;
  padding: 0;
  background: transparent;
  color: var(--p-text-color);
  font: inherit;
  font-weight: 600;
  text-align: left;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.status-name:disabled {
  cursor: default;
}
/* nhãn và nút cạnh tên giữ nguyên cỡ; tên dài thì tên co lại bằng "…" */
.default-pill,
.make-default,
.lock {
  flex: none;
}
.default-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.05rem 0.5rem;
  border-radius: 999px;
  background: var(--app-selected);
  color: var(--p-text-color);
  font-size: 0.75rem;
  font-weight: 600;
  white-space: nowrap;
}
.default-pill i {
  font-size: 0.7rem;
  color: var(--app-accent);
}
.make-default {
  font-size: 0.78rem;
  padding: 0.1rem 0.4rem;
}
/* "Make default" hiện khi rê chuột hay focus vào dòng, như nút hành động */
@media (hover: hover) {
  .lane-table :deep(tr) .make-default {
    opacity: 0;
  }
  .lane-table :deep(tr:hover) .make-default,
  .lane-table :deep(tr:focus-within) .make-default {
    opacity: 1;
  }
}
.lock {
  color: var(--p-text-muted-color);
  font-size: 0.8rem;
}
.archived-list {
  list-style: none;
  margin: 0;
  padding: 0.25rem 0 0;
  border-top: 1px dashed var(--app-line);
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.archived-list li {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding-left: 2.25rem;
}
.archived-name {
  color: var(--p-text-muted-color);
}
.archived-list .count {
  margin-left: auto;
}
.add-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem;
}
.add-row .p-inputtext {
  flex: 1;
  min-width: 10rem;
}
.add-row .field-error {
  flex-basis: 100%;
}
.add-btn {
  align-self: flex-start;
}
</style>
