<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Column from 'primevue/column'
import DataTable, { type DataTableRowReorderEvent } from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Tag from 'primevue/tag'
import { useConfirm } from 'primevue/useconfirm'
import { computed, nextTick, ref } from 'vue'
import IconAction from '@/components/IconAction.vue'
import PageHeader from '@/components/PageHeader.vue'
import type { Status, StatusKind } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import {
  kindSeverity,
  useArchiveStatus,
  useCreateStatus,
  useReorderStatuses,
  useRestoreStatus,
  useStatuses,
  useUpdateStatus,
} from '../api'
import { archiveBlock, KIND_INFO, KIND_ORDER, lanes, moveBy, orderAfterMove } from '../lanes'

// Status chia theo kind thành bốn làn; kéo thả (hay Alt+↑/↓ trên tên) đổi thứ tự trong làn
const session = useSession()
const confirm = useConfirm()
const canManage = computed(() => session.can(Perm.StatusManage))
const showArchived = ref(false)
const { data: statuses } = useStatuses(showArchived, true)

const byKind = computed(() => lanes(statuses.value ?? []))
const activeOf = (k: StatusKind) => byKind.value[k].filter((s) => !s.archived_at)
const archivedOf = (k: StatusKind) => byKind.value[k].filter((s) => s.archived_at)
const laneCount = (k: StatusKind) => activeOf(k).reduce((n, s) => n + (s.asset_count ?? 0), 0)

// Đổi thứ tự: gửi thứ tự của mọi status đang dùng
const reorder = useReorderStatuses()
function saveLane(kind: StatusKind, laneIds: string[]) {
  reorder.mutate(orderAfterMove(statuses.value ?? [], kind, laneIds))
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

const update = useUpdateStatus()
async function makeDefault(s: Status) {
  try {
    await update.mutateAsync({ id: s.id, make_default: true })
    notify.success(`${s.name} is now the default ${KIND_INFO[s.kind].label.toLowerCase()} status.`)
  } catch {
    // lỗi đã hiện qua toast của mutation
  }
}

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
    await create.mutateAsync({ name: newName.value, kind, position: last + 1 })
    notify.success(`${newName.value.trim()} added.`)
    adding.value = null
  } catch (err) {
    addErrors.set(err)
  }
}

// Sửa tên; kind không đổi được sau khi tạo
const editing = ref<Status | null>(null)
const editName = ref('')
const editErrors = useFormErrors()
const editOpen = computed({
  get: () => editing.value !== null,
  set: (v) => {
    if (!v) editing.value = null
  },
})
function openEdit(s: Status) {
  editing.value = s
  editName.value = s.name
  editErrors.clear()
}
async function submitEdit() {
  const s = editing.value
  if (!s) return
  editErrors.clear()
  try {
    await update.mutateAsync({ id: s.id, name: editName.value })
    editing.value = null
    notify.success('Status saved.')
  } catch (err) {
    editErrors.set(err)
  }
}

const archive = useArchiveStatus()
function askArchive(s: Status) {
  const n = s.asset_count ?? 0
  confirm.require({
    message: `Archive ${s.name}? ${n ? `The ${n} asset${n === 1 ? '' : 's'} using it keep it, but it` : 'It'} can't be chosen again until restored.`,
    header: 'Archive status',
    acceptLabel: 'Archive',
    rejectLabel: 'Cancel',
    acceptProps: { severity: 'danger' },
    accept: () =>
      archive
        .mutateAsync(s.id)
        .then(() => notify.success(`${s.name} archived.`))
        .catch(() => {}),
  })
}
const restore = useRestoreStatus()
function doRestore(s: Status) {
  restore
    .mutateAsync(s.id)
    .then(() => notify.success(`${s.name} restored.`))
    .catch(() => {})
}
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
          row-hover
          size="small"
          class="lane-table"
          table-style="width: 100%; table-layout: fixed"
          @row-reorder="(e: DataTableRowReorderEvent) => onReorder(k, e)"
        >
          <Column v-if="canManage" row-reorder header-style="width: 2.5rem" body-style="width: 2.5rem" />
          <Column>
            <template #body="{ data: s }: { data: Status }">
              <div class="name-cell">
                <button
                  class="status-name"
                  :data-status-name="s.id"
                  :disabled="!canManage"
                  :title="canManage ? 'Edit' : undefined"
                  @click="openEdit(s)"
                  @keydown.alt.up.prevent="moveKey(s, -1)"
                  @keydown.alt.down.prevent="moveKey(s, 1)"
                >
                  {{ s.name }}
                </button>
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
          <Column v-if="canManage" header-style="width: 5.5rem" body-style="width: 5.5rem">
            <template #body="{ data: s }: { data: Status }">
              <div class="row-actions">
                <IconAction icon="pi pi-pencil" label="Edit" @click="openEdit(s)" />
                <IconAction
                  icon="pi pi-inbox"
                  label="Archive"
                  danger
                  :disabled="!!archiveBlock(s)"
                  :reason="archiveBlock(s)"
                  @click="askArchive(s)"
                />
              </div>
            </template>
          </Column>
          <template #empty>No {{ KIND_INFO[k].label.toLowerCase() }} statuses.</template>
        </DataTable>

        <ul v-if="showArchived && archivedOf(k).length" class="archived-list">
          <li v-for="s in archivedOf(k)" :key="s.id">
            <span class="archived-name">{{ s.name }}</span>
            <Tag value="Archived" severity="secondary" />
            <span class="count">{{ s.asset_count ?? '' }}</span>
            <Button v-if="canManage" label="Restore" icon="pi pi-replay" text size="small" @click="doRestore(s)" />
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

    <Dialog v-model:visible="editOpen" modal header="Edit status" :style="{ width: '28rem' }">
      <form v-if="editing" class="form" @submit.prevent="submitEdit">
        <Message v-if="editErrors.general.value" severity="error">{{ editErrors.general.value }}</Message>
        <div class="field">
          <label for="status-name">Name</label>
          <InputText id="status-name" v-model="editName" required maxlength="100" autofocus />
          <small v-if="editErrors.fields.value.name" class="field-error">{{ editErrors.fields.value.name }}</small>
        </div>
        <div class="field">
          <span>Kind</span>
          <div><Tag :value="KIND_INFO[editing.kind].label" :severity="kindSeverity(editing.kind)" /></div>
          <small>The kind can't change after a status is created. Drag it in its lane to change the order.</small>
        </div>
        <div class="actions">
          <Button type="submit" label="Save" :loading="update.isPending.value" />
          <Button label="Cancel" severity="secondary" text @click="editing = null" />
        </div>
      </form>
    </Dialog>
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
