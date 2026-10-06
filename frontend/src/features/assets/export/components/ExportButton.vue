<script setup lang="ts">
import SplitButton from 'primevue/splitbutton'
import type { MenuItem } from 'primevue/menuitem'
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useExportProfiles } from '../api'
import { useExport } from '../useExport'
import type { ExportScope } from '../usePreviewData'

// Nút Export: bấm chính tải export dữ liệu theo bộ lọc hiện tại; mũi tên mở báo cáo,
// profile gần đây (tải ngay) và trang quản lý profile
const props = defineProps<{ scope: ExportScope }>()
const emit = defineEmits<{ report: [profileId?: string] }>()
const router = useRouter()
const { data: profiles } = useExportProfiles(true)
const { run, running } = useExport()

function dataExport() {
  run({ mode: 'data', filters: props.scope.filters }, 'storeit-assets.xlsx')
}
const items = computed<MenuItem[]>(() => [
  { label: 'Export report…', icon: 'pi pi-file-edit', disabled: running.value, command: () => emit('report') },
  { label: 'Data export (.xlsx)', icon: 'pi pi-table', disabled: running.value, command: dataExport },
  ...(profiles.value?.length ? [{ separator: true }] : []),
  ...(profiles.value ?? []).slice(0, 3).map((p) => ({
    label: p.name,
    icon: 'pi pi-bolt',
    disabled: running.value,
    command: () => run({ mode: 'report', filters: props.scope.filters, profile_id: p.id }, `${p.name}.xlsx`),
  })),
  { separator: true },
  { label: 'Manage profiles…', icon: 'pi pi-cog', command: () => router.push('/export-profiles') },
])
</script>

<template>
  <SplitButton label="Export" icon="pi pi-download" severity="secondary" outlined :model="items" :loading="running" @click="dataExport" />
</template>
