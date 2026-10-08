<script setup lang="ts">
import SplitButton from 'primevue/splitbutton'
import type { MenuItem } from 'primevue/menuitem'
import { computed, ref, type ButtonHTMLAttributes } from 'vue'
import { useRouter } from 'vue-router'
import KeyHint from '@/components/KeyHint.vue'
import { useSession } from '@/lib/auth/session'
import { usePageKeys } from '@/lib/pageKeys'
import { useExportProfiles } from '../api'
import { useExport } from '../useExport'
import type { ExportScope } from '../usePreviewData'
import DataExportDialog from './DataExportDialog.vue'

// Nút Export: bấm chính (phím X) tải export dữ liệu theo bộ lọc hiện tại; mũi tên mở báo cáo
// (phím R), profile gần đây (tải ngay) và trang quản lý profile
defineOptions({ inheritAttrs: false })
const props = defineProps<{ scope: ExportScope }>()
const emit = defineEmits<{ report: [profileId?: string] }>()
const router = useRouter()
const session = useSession()
const { data: profiles } = useExportProfiles(true)
const { run, running } = useExport()

// "What’s inside" trong thông báo sau khi tải mở bản xem trước của file dữ liệu; phạm vi
// chụp lúc bấm để vẫn đúng khi danh sách đã đổi bộ lọc
const insideScope = ref<ExportScope | null>(null)
const insideOpen = computed({
  get: () => insideScope.value !== null,
  set: (open) => {
    if (!open) insideScope.value = null
  },
})
function dataExport() {
  const scope = props.scope
  run({ mode: 'data', filters: scope.filters }, 'storeit-assets.xlsx', { rows: scope.count, inside: () => (insideScope.value = scope) })
}
// SplitButton không có prop loading: chuyển xuống nút chính (PrimeVue Button) qua buttonProps
const mainButtonProps = computed(() => ({ loading: running.value, 'aria-label': 'Export data (X)' }) as ButtonHTMLAttributes)
// Phím: X tải export dữ liệu, R mở báo cáo (như hai mục đầu của menu)
usePageKeys((e) => {
  if (running.value) return
  if (e.key === 'x') dataExport()
  else if (e.key === 'r') emit('report')
})
// ba profile mới sửa gần nhất
const recent = computed(() => [...(profiles.value ?? [])].sort((a, b) => b.updated_at.localeCompare(a.updated_at)).slice(0, 3))
const items = computed<MenuItem[]>(() => [
  { label: 'Export report…', icon: 'pi pi-file-edit', shortcut: 'R', disabled: running.value, command: () => emit('report') },
  { label: 'Data export (.xlsx)', icon: 'pi pi-table', meta: 're-importable', shortcut: 'X', disabled: running.value, command: dataExport },
  ...(recent.value.length ? [{ separator: true }, { label: 'Profiles', heading: true, disabled: true }] : []),
  ...recent.value.map((p) => ({
    label: p.name,
    icon: 'pi pi-bolt',
    meta: p.owner.id === session.me?.account.id ? 'Mine' : p.shared ? 'Shared' : '',
    disabled: running.value,
    command: () => run({ mode: 'report', filters: props.scope.filters, profile_id: p.id }, `${p.name}.xlsx`, { rows: props.scope.count }),
  })),
  { separator: true },
  { label: 'Manage profiles…', icon: 'pi pi-cog', command: () => router.push('/export-profiles') },
])
</script>

<template>
  <SplitButton v-bind="$attrs" label="Export" icon="pi pi-download" severity="secondary" outlined :model="items" :button-props="mainButtonProps" @click="dataExport">
    <i :class="running ? 'pi pi-spin pi-spinner' : 'pi pi-download'" aria-hidden="true" />
    <span>Export</span>
    <KeyHint keys="X" />
    <template #item="{ item, props: p }">
      <div v-if="item.heading" class="menu-heading">{{ item.label }}</div>
      <a v-else v-bind="p.action" class="menu-link">
        <span :class="['menu-icon', item.icon]" aria-hidden="true" />
        <span class="menu-label">{{ item.label }}</span>
        <span v-if="item.meta" class="menu-meta">{{ item.meta }}</span>
        <KeyHint v-if="item.shortcut" :keys="item.shortcut" />
      </a>
    </template>
  </SplitButton>
  <DataExportDialog v-if="insideScope" v-model:visible="insideOpen" :scope="insideScope" />
</template>

<style scoped>
.menu-heading {
  padding: 0.35rem 0.75rem 0.15rem;
  font-size: 0.7rem;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--p-text-muted-color);
}
.menu-link {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  color: inherit;
  text-decoration: none;
  cursor: pointer;
}
.menu-icon {
  color: var(--p-text-muted-color);
}
.menu-label {
  flex: 1;
  white-space: nowrap;
}
.menu-meta {
  margin-left: 1rem;
  font-size: 0.75rem;
  color: var(--p-text-muted-color);
}
</style>
