<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Popover from 'primevue/popover'
import Select from 'primevue/select'
import Tag from 'primevue/tag'
import { computed, ref, watch } from 'vue'
import FormDialog from '@/components/FormDialog.vue'
import { announce } from '@/lib/actions'
import type { ExportLayout, ExportProfile } from '@/lib/api/types'
import { useSession } from '@/lib/auth/session'
import { useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { useCreateExportProfile, useExportProfiles, useUpdateExportProfile } from '../api'
import { cleanSheetName, defaultReportLayout, exportFileName, normalizeLayout, profilePatch } from '../layout'
import { useExport } from '../useExport'
import { type ExportScope, usePreviewData } from '../usePreviewData'
import ReportEditor from './ReportEditor.vue'

// Hộp thoại "Export report": chọn profile, chỉnh cột và định dạng, xem trước, tải về
const props = defineProps<{ scope: ExportScope; profileId?: string }>()
const visible = defineModel<boolean>('visible', { required: true })

const session = useSession()
const { data: profiles } = useExportProfiles(true)
const { types, rows } = usePreviewData(
  () => props.scope,
  () => visible.value,
)

const profileId = ref<string | null>(null)
const profile = computed<ExportProfile | undefined>(() => profiles.value?.find((p) => p.id === profileId.value))
// bố cục đưa vào trình sửa (nạp lại khi sourceKey đổi) và bố cục đang sửa nó báo lên
const source = ref<ExportLayout>(defaultReportLayout())
const sourceKey = ref(0)
const current = ref<ExportLayout>(defaultReportLayout())
// bố cục đã lưu của profile// bố cục đã lưu của profile đang mở (null khi chưa chọn profile)
const saved = ref<string | null>(null)
// version của profile lúc nạp vào trình sửa: Save gửi version này (không phải bản mới nhất
// của danh sách profile) nên sửa đè lên thay đổi của người khác vẫn bị 409
const loadedVersion = ref<number | null>(null)

// load: nạp bố cục của profile (hay bố cục mặc định) và dựng lại cột của trình sửa.
// Chọn profile mà danh sách chưa về thì chờ, watcher bên dưới gọi lại khi có.
function load() {
  if (!visible.value) return
  const p = profile.value
  if (profileId.value && !p) return
  source.value = p ? p.layout : { ...defaultReportLayout(), sheet_name: cleanSheetName(props.scope.label.slice(0, 31)) || 'Assets' }
  saved.value = p ? normalizeLayout(p.layout) : null
  loadedVersion.value = p?.version ?? null
  sourceKey.value++
}
watch(visible, (open) => {
  if (!open) return
  profileId.value = props.profileId ?? null
  load()
})
watch([profileId, () => profile.value?.id], load)
const dirty = computed(() => saved.value !== null && normalizeLayout(current.value) !== saved.value)
// Có cột nào để xuất không (mỗi loại một sheet có thể chỉ dùng thuộc tính riêng của loại)
const hasColumns = computed(() => current.value.columns.length > 0 || (current.value.sheets === 'per_type' && !!current.value.each_type_attrs))
const fileName = computed(() => exportFileName('report', profile.value?.name, new Date().toISOString().slice(0, 10)))

// Tab cấu hình đang mở (cột, bố cục, định dạng); giữ nguyên khi đóng mở lại hộp thoại
const tab = ref<'columns' | 'layout' | 'format'>('columns')

// Save…: một popover cho cả lưu vào profile này (tên, chia sẻ, cột và định dạng trong một
// PATCH) và lưu bản mới. Profile không sửa được, hay chưa chọn profile: chỉ Save as new.
const update = useUpdateExportProfile(false)
const create = useCreateExportProfile()
const errors = useFormErrors()
const saveForm = ref<{ name: string; shared: boolean; hint: string } | null>(null)
const savePop = ref<InstanceType<typeof Popover>>()
const canSaveHere = computed(() => !!profile.value?.can_edit)
// thay đổi Save sẽ gửi; null thì nút Save tắt
const patch = computed(() => {
  const p = profile.value
  if (!p || !saveForm.value) return null
  return profilePatch(p, saveForm.value, dirty.value ? current.value : null)
})
function openSave(e: Event) {
  errors.clear()
  const p = profile.value
  saveForm.value = p?.can_edit ? { name: p.name, shared: p.shared, hint: '' } : { name: p ? `${p.name} (copy)` : 'New report', shared: false, hint: '' }
  savePop.value?.toggle(e)
}
function closeSave() {
  savePop.value?.hide()
  saveForm.value = null
}
async function saveHere() {
  const p = profile.value
  const ch = patch.value
  if (!p || !ch || loadedVersion.value === null) return
  errors.clear()
  try {
    // bản trước lần lưu này, để Undo đặt lại
    const before = { name: p.name, shared: p.shared, layout: p.layout }
    const next = await update.mutateAsync({ id: p.id, version: loadedVersion.value, ...ch })
    saved.value = normalizeLayout(next.layout)
    loadedVersion.value = next.version
    closeSave()
    announce(next, {
      done: ch.name ? `Saved ${next.name} (renamed).` : `Saved ${next.name}.`,
      undo: async (n) => {
        const back = await update.mutateAsync({ id: p.id, version: n.version, name: before.name, shared: before.shared, layout: before.layout })
        // hộp thoại còn mở trên profile này: lấy bản vừa đặt lại
        if (profileId.value === p.id) {
          saved.value = normalizeLayout(back.layout)
          loadedVersion.value = back.version
        }
      },
      undone: `${before.name} put back as it was.`,
      undoFailed: `Couldn't put ${before.name} back. The saved version stays.`,
    })
  } catch (err) {
    errors.set(err)
  }
}
async function saveNew() {
  const f = saveForm.value
  if (!f) return
  const p = profile.value
  // tên chưa đổi trên profile của mình: gợi ý "(copy)" thay vì báo trùng tên
  if (p && p.owner.id === session.me?.account.id && f.name.trim() === p.name) {
    f.name = `${p.name} (copy)`
    f.hint = 'Pick a name for the copy, then Save as new again.'
    return
  }
  errors.clear()
  try {
    const np = await create.mutateAsync({ name: f.name, shared: f.shared, layout: current.value })
    closeSave()
    profileId.value = np.id
    notify.success(`Saved ${np.name}.`)
  } catch (err) {
    errors.set(err)
  }
}
// Enter trong ô tên: Save khi lưu được vào profile này, không thì Save as new
function submitSave() {
  if (canSaveHere.value && patch.value) saveHere()
  else saveNew()
}

const { run, running } = useExport()
async function download() {
  // lỗi (vd quá số dòng) thì giữ hộp thoại để không mất phần đã sửa
  const ok = await run({ mode: 'report', filters: props.scope.filters, layout: current.value, profile_id: profileId.value ?? undefined }, 'storeit-report.xlsx', {
    rows: props.scope.count,
  })
  if (ok) visible.value = false
}

</script>

<template>
  <FormDialog v-model:visible="visible" icon="file" title="Export report" width="max(80rem, 88vw)" flush :dirty="dirty" class="report-dialog">
    <!-- Cạnh tiêu đề: profile, trạng thái, lưu; không thêm hàng nào trên vùng cuộn -->
    <template #header-extra>
      <div class="title-bar">
        <Select v-model="profileId" aria-label="Export profile" :options="profiles ?? []" option-label="name" option-value="id" placeholder="No profile" show-clear size="small" class="profile-select" />
        <i v-if="profile && !profile.can_edit" v-tooltip.bottom="`Shared by ${profile.owner.name}`" class="pi pi-lock state" aria-label="Shared by someone else" />
        <i v-else-if="profile?.shared" v-tooltip.bottom="'Shared with everyone who can export'" class="pi pi-users state" aria-label="Shared" />
        <Tag v-if="dirty" value="Unsaved" severity="warn" class="unsaved" />
        <Button label="Save…" size="small" text aria-haspopup="dialog" @click="openSave" />
      </div>
    </template>
    <Popover ref="savePop" @hide="saveForm = null">
      <form v-if="saveForm" class="save-as" @submit.prevent="submitSave">
        <label for="save-name">Name</label>
        <InputText id="save-name" v-model="saveForm.name" required maxlength="100" autofocus fluid :invalid="!!errors.fields.value.name" @update:model-value="saveForm.hint = ''" />
        <span class="check">
          <Checkbox v-model="saveForm.shared" input-id="save-shared" binary />
          <label for="save-shared">Share with everyone who can export</label>
        </span>
        <Message v-if="errors.general.value || errors.fields.value.name" severity="error" size="small" variant="simple">{{ errors.fields.value.name ?? errors.general.value }}</Message>
        <small v-if="saveForm.hint" class="save-hint">{{ saveForm.hint }}</small>
        <div class="save-as-actions">
          <Button label="Cancel" size="small" text severity="secondary" @click="closeSave" />
          <Button label="Save as new" size="small" :severity="canSaveHere ? 'secondary' : undefined" :outlined="canSaveHere" :loading="create.isPending.value" @click="saveNew" />
          <Button v-if="canSaveHere" label="Save" size="small" :disabled="!patch" :loading="update.isPending.value" @click="saveHere" />
        </div>
      </form>
    </Popover>

    <div class="report">
      <ReportEditor
        v-model:tab="tab"
        :source="source"
        :source-key="sourceKey"
        :types="types"
        :rows="rows"
        :scope-label="scope.label"
        :row-count="scope.count"
        :file-name="fileName"
        :title-name="profile?.name"
        @update:current="(l) => (current = l)"
      />
    </div>
    <template #footer="{ close }">
      <span v-tooltip.top="'Built on the server with the same filters as the list. Up to 50,000 rows.'" class="foot-note">
        <i class="pi pi-table" aria-hidden="true" />
        <span><b>{{ scope.count }}</b> {{ scope.count === 1 ? 'asset' : 'assets' }} · {{ scope.label }}</span>
        <span class="muted">· {{ scope.selection ? 'only the selected assets' : 'uses the list’s current filters' }}</span>
      </span>
      <!-- qua FormDialog: còn thay đổi chưa lưu thì hỏi như ✕ và Esc -->
      <Button label="Cancel" text severity="secondary" @click="close" />
      <Button label="Download .xlsx" icon="pi pi-download" :loading="running" :disabled="!hasColumns" @click="download" />
    </template>
  </FormDialog>
</template>

<style scoped>
.title-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem 0.6rem;
  flex: 1;
  min-width: 0;
}
.state {
  color: var(--p-text-muted-color);
}
.unsaved {
  font-size: 0.72rem;
}
.save-as {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  width: min(20rem, 80vw);
}
.save-lead {
  font-size: 0.84rem;
  color: var(--p-text-muted-color);
}
.save-hint {
  color: var(--p-primary-color);
}
.save-as-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.4rem;
}
.profile-select {
  min-width: 13rem;
}
/* Chiều cao cố định: thanh profile và dòng phạm vi đứng yên, cột cấu hình và
   bản xem trước mỗi bên tự cuộn */
.report {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.check {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
}
.muted {
  color: var(--p-text-muted-color);
}
.foot-note {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.45rem;
  margin-right: auto;
  text-align: left;
  font-size: 0.88rem;
  line-height: 1.2;
}
/* Màn hẹp: cả hộp thoại cuộn như cũ */
@media (max-width: 900px) {
  .report {
    height: auto;
  }
}
</style>

<style>
/* Hộp thoại cao cố định, nội dung không cuộn: chỉ cột cấu hình và bản xem trước cuộn.
   Dialog teleport ra body nên khối này không scoped. Màn hẹp: cả hộp thoại cuộn như cũ. */
.p-dialog.report-dialog {
  height: min(50rem, 94vh);
}
.p-dialog.report-dialog .p-dialog-content {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}
@media (max-width: 900px) {
  .p-dialog.report-dialog {
    height: auto;
  }
  .p-dialog.report-dialog .p-dialog-content {
    overflow: auto;
  }
}
</style>
