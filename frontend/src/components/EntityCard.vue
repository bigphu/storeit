<script setup lang="ts">
import Card from 'primevue/card'
import { type RouteLocationRaw, useRouter } from 'vue-router'
import { openLocation, wantsNewTab } from '@/lib/navigation'
import { isRowControl } from '@/lib/tableRows'

// Thẻ bấm được trong CardGrid: bấm (hay Enter) mở `to`, Ctrl/⌘ hay chuột giữa mở tab mới.
// Bấm trúng nút, liên kết bên trong thì để phần tử đó tự xử lý, như dòng bảng.
const props = defineProps<{ to: RouteLocationRaw; label: string; dimmed?: boolean }>()
const emit = defineEmits<{ menu: [e: MouseEvent] }>()
const router = useRouter()

function open(e?: MouseEvent) {
  openLocation(router, props.to, e)
}
function onClick(e: MouseEvent) {
  if (!isRowControl(e.target)) open(e)
}
function onAux(e: MouseEvent) {
  if (wantsNewTab(e) && !isRowControl(e.target)) open(e)
}
</script>

<template>
  <Card
    class="entity-card"
    :class="{ dimmed }"
    tabindex="0"
    role="link"
    :aria-label="label"
    @click="onClick"
    @auxclick="onAux"
    @mousedown.middle.prevent
    @keydown.enter.self="open()"
    @contextmenu="(e: MouseEvent) => emit('menu', e)"
  >
    <template #content>
      <div class="entity-card-body"><slot /></div>
    </template>
  </Card>
</template>

<style scoped>
.entity-card {
  cursor: pointer;
  border: 1px solid var(--app-line);
  border-radius: 12px;
  box-shadow: none;
  transition: box-shadow 0.15s ease;
}
.entity-card:hover,
.entity-card:focus-visible {
  box-shadow: 0 0 0 1px var(--app-accent);
}
.entity-card.dimmed {
  opacity: 0.8;
}
.entity-card :deep(.p-card-body) {
  padding: 0.95rem;
}
.entity-card-body {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
}
</style>
