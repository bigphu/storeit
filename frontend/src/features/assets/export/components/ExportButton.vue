<script setup lang="ts">
import SplitButton from 'primevue/splitbutton'
import type { MenuItem } from 'primevue/menuitem'
import { computed, type ButtonHTMLAttributes } from 'vue'
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
// SplitButton không có prop loading: chuyển xuống nút chính (PrimeVue Button) qua buttonProps
const mainButtonProps = computed(() => ({ loading: running.value }) as ButtonHTMLAttributes)
// ba profile mới sửa gần nhất
const recent = computed(() => [...(profiles.value ?? [])].sort((a, b) => b.updated_at.localeCompare(a.updated_at)).slice(0, 3))
const items = computed<MenuItem[]>(() => [
  { label: 'Export report…', icon: 'pi pi-file-edit', disabled: running.value, command: () => emit('report') },
  { label: 'Data export (.xlsx)', icon: 'pi pi-table', disabled: running.value, command: dataExport },
  ...(recent.value.length ? [{ separator: true }] : []),
  ...recent.value.map((p) => ({
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
  <SplitButton label="Export" icon="pi pi-download" severity="secondary" outlined :model="items" :button-props="mainButtonProps" @click="dataExport" />
</template>
