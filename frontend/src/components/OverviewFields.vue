<script lang="ts">
// FieldDef: một trường của Overview (trang chi tiết và ngăn kéo sửa nhanh)
export interface FieldDef {
  key: string
  label: string
  kind?: 'text' | 'textarea' | 'select' | 'date'
  options?: { label: string; value: string }[]
  // không đổi được: hiện chữ, ổ khoá và lý do
  lock?: string
  maxlength?: number
}
</script>

<script setup lang="ts">
import DatePicker from 'primevue/datepicker'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'
import { watch } from 'vue'
import { formatDate, fromDateString, toDateString } from '@/lib/dates'
import { type Draft, type DraftValue, setEdit, valueOf, pruneTab } from '@/lib/detailDraft'

// Lưới nhãn/giá trị; trường sửa được là ô nhập ngay từ đầu (không có nút Edit). Đổi giá trị
// ghi vào bản nháp; trường đã đổi tô nền cam nhạt. Chỉ xem: mọi trường là chữ
const props = withDefaults(
  defineProps<{
    fields: FieldDef[]
    saved: Record<string, DraftValue>
    draft: Draft
    tab?: string
    readonly?: boolean
    errors?: Record<string, string>
    stacked?: boolean
  }>(),
  { tab: 'overview' },
)
const savedOf = (f: FieldDef) => props.saved[f.key] ?? ''
// giá trị đã lưu đổi (lưu ở chỗ khác, dữ liệu nạp lại): thay đổi nay trùng thì không còn đếm
watch(
  () => props.saved,
  (s) => pruneTab(props.draft, props.tab, s),
  { deep: true },
)
const val = (f: FieldDef) => valueOf(props.draft, props.tab, f.key, savedOf(f))
const changed = (f: FieldDef) => val(f) !== savedOf(f)
const set = (f: FieldDef, v: DraftValue | undefined) => setEdit(props.draft, props.tab, f.key, v ?? '', savedOf(f))
const shown = (f: FieldDef) => {
  const v = String(val(f) ?? '')
  if (f.kind === 'select') return f.options?.find((o) => o.value === v)?.label ?? v
  if (f.kind === 'date') return v ? formatDate(v) : '—'
  return v || '—'
}
</script>

<template>
  <div :class="['overview-fields', { stacked }]">
    <template v-for="f in fields" :key="f.key">
      <template v-if="f.lock || readonly">
        <span class="lbl">{{ f.label }}</span>
        <div class="ro">
          <span class="ro-value">{{ shown(f) }}</span>
          <span v-if="f.lock" class="lock"><i class="pi pi-lock" aria-hidden="true" />{{ f.lock }}</span>
        </div>
      </template>
      <template v-else>
        <label :for="`of-${f.key}`" class="lbl">{{ f.label }}</label>
        <div class="cell">
          <Textarea
            v-if="f.kind === 'textarea'"
            :id="`of-${f.key}`"
            :model-value="String(val(f) ?? '')"
            auto-resize
            rows="2"
            :maxlength="f.maxlength"
            :class="{ changed: changed(f) }"
            fluid
            @update:model-value="(v) => set(f, v)"
          />
          <Select
            v-else-if="f.kind === 'select'"
            :input-id="`of-${f.key}`"
            :model-value="val(f)"
            :options="f.options"
            option-label="label"
            option-value="value"
            :class="{ changed: changed(f) }"
            fluid
            @update:model-value="(v) => set(f, v)"
          />
          <DatePicker
            v-else-if="f.kind === 'date'"
            :input-id="`of-${f.key}`"
            :model-value="fromDateString(String(val(f) ?? '')) ?? undefined"
            date-format="dd/mm/yy"
            show-button-bar
            :class="{ changed: changed(f) }"
            fluid
            @update:model-value="(v) => set(f, toDateString(v as Date | null) ?? '')"
          />
          <InputText
            v-else
            :id="`of-${f.key}`"
            :model-value="String(val(f) ?? '')"
            :maxlength="f.maxlength"
            :class="{ changed: changed(f) }"
            fluid
            @update:model-value="(v) => set(f, v)"
          />
          <small v-if="errors?.[f.key]" class="field-error">{{ errors[f.key] }}</small>
        </div>
      </template>
    </template>
  </div>
</template>

<style scoped>
.overview-fields {
  display: grid;
  grid-template-columns: 10rem minmax(0, 1fr);
  gap: 0.85rem 1.2rem;
  align-items: start;
  max-width: 44rem;
}
.overview-fields.stacked {
  grid-template-columns: minmax(0, 1fr);
  gap: 0.3rem;
}
.overview-fields.stacked .lbl {
  padding-top: 0.6rem;
}
.lbl {
  padding-top: 0.5rem;
  font-size: 0.85rem;
  color: var(--p-text-muted-color);
}
.cell {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-width: 0;
}
.ro {
  padding-top: 0.5rem;
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
  color: var(--p-text-color);
}
.ro-value {
  overflow-wrap: anywhere;
}
.lock {
  display: inline-flex;
  gap: 0.3rem;
  align-items: center;
  font-size: 0.8rem;
  color: var(--p-text-muted-color);
}
.lock i {
  font-size: 0.75rem;
}
/* trường đã đổi: nền cam nhạt, viền cam đủ bốn cạnh */
.cell :deep(.changed),
.cell :deep(.changed input) {
  background: var(--app-warn-soft);
  border-color: color-mix(in srgb, var(--app-warn) 55%, var(--app-line));
}
@media (max-width: 640px) {
  .overview-fields {
    grid-template-columns: minmax(0, 1fr);
    gap: 0.3rem;
  }
}
</style>
