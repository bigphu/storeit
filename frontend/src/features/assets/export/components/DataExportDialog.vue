<script setup lang="ts">
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import { computed } from 'vue'
import { dataPreviewLayout, dataPreviewSheets, exportFileName } from '../layout'
import { type ExportScope, usePreviewData } from '../usePreviewData'
import SheetPreview from './SheetPreview.vue'

// "What’s in the data export": xem trước file dữ liệu vừa tải (mở từ thông báo sau khi
// tải). Mỗi loại một sheet theo code, tiêu đề là khoá trường, ngày ISO, TRUE/FALSE,
// để sửa rồi import lại được
const props = defineProps<{ scope: ExportScope }>()
const visible = defineModel<boolean>('visible', { required: true })

const { types, rows } = usePreviewData(
  () => props.scope,
  () => visible.value,
)
const layout = dataPreviewLayout()
const sheets = computed(() => dataPreviewSheets(rows.value, types.value))
const fileName = computed(() => exportFileName('data', undefined, new Date().toISOString().slice(0, 10)))
</script>

<template>
  <Dialog v-model:visible="visible" modal header="What’s in the data export" :style="{ width: 'min(60rem, 96vw)' }" :content-style="{ padding: 0 }">
    <div class="data-export">
      <div class="sub">
        <i class="pi pi-table" aria-hidden="true" />
        <span>
          <b>{{ scope.count }}</b> {{ scope.count === 1 ? 'asset' : 'assets' }} · {{ scope.label }} · {{ sheets.length }} {{ sheets.length === 1 ? 'sheet' : 'sheets' }}, one per asset type
        </span>
      </div>
      <div class="body">
        <p class="note">Headers are field keys and dates are ISO, so the file can be edited and imported back. No title row and no styling apart from a frozen header.</p>
        <SheetPreview class="fill" :sheets="sheets" :layout="layout" :types="types" :title="[]" />
      </div>
    </div>
    <template #footer>
      <span class="file">{{ fileName }}</span>
      <Button label="Done" @click="visible = false" />
    </template>
  </Dialog>
</template>

<style scoped>
.data-export {
  display: flex;
  flex-direction: column;
  height: min(36rem, 70vh);
}
.sub {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  padding: 0.7rem 1.1rem;
  border-bottom: 1px solid var(--app-line);
  background: var(--app-ground);
  font-size: 0.88rem;
}
.body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  padding: 0.9rem 1.1rem;
  background: var(--app-ground);
}
.body > .fill {
  flex: 1 1 auto;
  min-height: 0;
}
.note {
  font-size: 0.88rem;
}
.file {
  margin-right: auto;
  font: 0.82rem var(--app-mono);
  color: var(--p-text-muted-color);
}
@media (max-width: 900px) {
  .data-export {
    height: auto;
  }
  .body :deep(.sheet-scroll) {
    max-height: 26rem;
  }
}
</style>
