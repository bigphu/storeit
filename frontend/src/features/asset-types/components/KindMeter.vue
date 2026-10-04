<script setup lang="ts">
import MeterGroup from 'primevue/metergroup'
import { computed } from 'vue'
import type { AssetType } from '@/lib/api/types'

// Thanh tài sản theo kind của một loại: xanh có thể giao, xanh dương đang dùng, cam không
// dùng được. legend: dòng chú thích màu (đặt một lần trên trang, không phải mỗi thẻ).
const props = defineProps<{ type?: AssetType; legend?: boolean }>()

const COLORS = {
  available: 'var(--p-green-500)',
  in_use: 'var(--p-sky-500)',
  unavailable: 'var(--p-orange-500)',
}
const LABELS = { available: 'Available', in_use: 'In use', unavailable: 'Unavailable' }
const KEYS = ['available', 'in_use', 'unavailable'] as const

const value = computed(() => {
  const k = props.type?.kind_counts
  return k ? KEYS.map((key) => ({ label: LABELS[key], value: k[key], color: COLORS[key] })) : []
})
const title = computed(() => {
  const k = props.type?.kind_counts
  return k ? `${k.available} available · ${k.in_use} in use · ${k.unavailable} unavailable` : ''
})
</script>

<template>
  <div v-if="legend" class="legend" aria-hidden="true">
    <span v-for="key in KEYS" :key="key"><i :style="{ background: COLORS[key] }" />{{ LABELS[key] }}</span>
  </div>
  <div v-else :title="title">
    <MeterGroup :value="value" :max="Math.max(type?.asset_count ?? 0, 1)" class="kind-meter">
      <template #label><span /></template>
    </MeterGroup>
  </div>
</template>

<style scoped>
.kind-meter :deep(.p-metergroup-label-list) {
  display: none;
}
.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem 0.9rem;
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
</style>
