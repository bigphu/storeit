<script setup lang="ts">
import Button from 'primevue/button'
import Chip from 'primevue/chip'
import Select from 'primevue/select'
import SelectButton from 'primevue/selectbutton'
import ToggleSwitch from 'primevue/toggleswitch'
import { computed } from 'vue'
import { PAGE_SIZES, tableLabel, usePreferences, withTableSize } from '@/lib/preferences'
import { useAssetTypes } from '@/features/asset-types/api'

// Tuỳ chọn: áp ngay khi đổi, lưu theo tài khoản trên máy này
const store = usePreferences()
const { data: types } = useAssetTypes(true)

const themes = [
  { label: 'Match my device', value: 'system' },
  { label: 'Light', value: 'light' },
  { label: 'Dark', value: 'dark' },
]
const densities = [
  { label: 'Comfortable', value: 'comfortable' },
  { label: 'Compact', value: 'compact' },
]

// Các bảng đã chọn số dòng riêng (khác mặc định)
const ownSizes = computed(() =>
  Object.entries(store.prefs.tableSizes).map(([key, size]) => ({
    key,
    size,
    label: tableLabel(key, (id) => types.value?.find((t) => t.id === id)?.name),
  })),
)

function useDefault(key: string) {
  store.prefs = withTableSize(store.prefs, key, 'default')
}
function useDefaultEverywhere() {
  store.prefs = { ...store.prefs, tableSizes: {} }
}
</script>

<template>
  <div class="form">
    <div class="field">
      <span id="pref-theme">Theme</span>
      <SelectButton
        v-model="store.prefs.theme"
        :options="themes"
        option-label="label"
        option-value="value"
        :allow-empty="false"
        aria-labelledby="pref-theme"
      />
    </div>
    <div class="field">
      <span id="pref-density">Table density</span>
      <SelectButton
        v-model="store.prefs.density"
        :options="densities"
        option-label="label"
        option-value="value"
        :allow-empty="false"
        aria-labelledby="pref-density"
      />
    </div>
    <div class="field">
      <label for="pref-rows">Default rows per page</label>
      <Select v-model="store.prefs.defaultPageSize" input-id="pref-rows" :options="PAGE_SIZES" class="narrow" />
      <small>Used by every table where you haven't picked a size. Each table also has its own rows control under it.</small>
    </div>
    <div class="toggle-row">
      <ToggleSwitch v-model="store.prefs.reopenTabs" input-id="pref-reopen" />
      <label for="pref-reopen">
        Reopen my tabs when I sign in
        <small>Off: only pinned tabs come back.</small>
      </label>
    </div>
    <div v-if="ownSizes.length" class="field">
      <span>Tables with their own size</span>
      <div class="actions">
        <Chip
          v-for="t in ownSizes"
          :key="t.key"
          :label="`${t.label} · ${t.size}`"
          removable
          @remove="useDefault(t.key)"
        />
      </div>
      <span><Button label="Use the default everywhere" text size="small" @click="useDefaultEverywhere" /></span>
    </div>
    <small>Preferences are saved for your account on this device.</small>
  </div>
</template>

<style scoped>
.narrow {
  width: 8rem;
}
.toggle-row {
  display: flex;
  gap: 0.75rem;
  align-items: flex-start;
}
.toggle-row label {
  display: flex;
  flex-direction: column;
}
</style>
