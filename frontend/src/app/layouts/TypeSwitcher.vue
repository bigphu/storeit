<script setup lang="ts">
import Dialog from 'primevue/dialog'
import Listbox, { type ListboxChangeEvent } from 'primevue/listbox'
import { computed, nextTick, ref } from 'vue'
import { useRouter } from 'vue-router'
import { openLocation } from '@/lib/navigation'
import { usePreferences } from '@/lib/preferences'
import { useAssetTypes } from '@/features/asset-types/api'
import { useTypeNav } from '../useTypeNav'

// Bộ chọn loại (Ctrl K, nút ở sidebar): loại gần đây trước; Enter mở trong trang này,
// Ctrl+Enter / Ctrl-click mở tab mới
const visible = defineModel<boolean>('visible', { required: true })

const router = useRouter()
const prefs = usePreferences()
const nav = useTypeNav()
const { data: types } = useAssetTypes(false, true)

interface Option {
  id: string
  name: string
  count?: number
}
const groups = computed(() => {
  const all: Option[] = (types.value ?? []).map((t) => ({ id: t.id, name: t.name, count: t.asset_count }))
  const recent = prefs.prefs.recentTypes.flatMap((id) => all.filter((o) => o.id === id))
  const rest = all.filter((o) => !recent.includes(o))
  return [
    { label: 'Recent', items: recent },
    { label: 'All types', items: rest },
  ].filter((g) => g.items.length)
})

const picked = ref<Option | null>(null)
const wrap = ref<HTMLElement>()

function onShow() {
  picked.value = null
  nextTick(() => wrap.value?.querySelector('input')?.focus())
}

function pick(e: ListboxChangeEvent) {
  const o = e.value as Option | null
  if (!o) return
  visible.value = false
  openLocation(router, nav.locationFor(o.id), e.originalEvent as MouseEvent)
}
</script>

<template>
  <Dialog
    v-model:visible="visible"
    modal
    dismissable-mask
    :show-header="false"
    position="top"
    :style="{ width: 'min(92vw, 30rem)' }"
    @show="onShow"
  >
    <div ref="wrap">
      <Listbox
        v-model="picked"
        :options="groups"
        option-label="name"
        option-group-label="label"
        option-group-children="items"
        filter
        filter-placeholder="Go to asset type…"
        list-style="max-height: 50vh"
        class="switcher-list"
        @change="pick"
      >
        <template #option="{ option }">
          <span class="opt">
            <span>{{ option.name }}</span>
            <span class="count">{{ option.count ?? '' }}</span>
          </span>
        </template>
        <template #footer>
          <small class="hint">Enter opens here · Ctrl+Enter or Ctrl-click opens a new tab</small>
        </template>
      </Listbox>
    </div>
  </Dialog>
</template>

<style scoped>
.switcher-list {
  border: 0;
  width: 100%;
}
.opt {
  display: flex;
  justify-content: space-between;
  width: 100%;
  gap: 1rem;
}
.count {
  color: var(--p-text-muted-color);
  font-variant-numeric: tabular-nums;
}
.hint {
  display: block;
  padding: 0.5rem 0.75rem;
  color: var(--p-text-muted-color);
}
</style>
