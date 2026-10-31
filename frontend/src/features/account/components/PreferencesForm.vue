<script setup lang="ts">
import Button from 'primevue/button'
import Chip from 'primevue/chip'
import Select from 'primevue/select'
import ToggleSwitch from 'primevue/toggleswitch'
import { computed } from 'vue'
import SegmentedFilter from '@/components/SegmentedFilter.vue'
import SettingsGroup from '@/components/SettingsGroup.vue'
import SettingsRow from '@/components/SettingsRow.vue'
import { PAGE_SIZES, tableLabel, usePreferences, withTableSize } from '@/lib/preferences'
import { useAssetTypes } from '@/features/asset-types/api'

// Tuỳ chọn: áp ngay khi đổi, lưu theo tài khoản trên máy này
const store = usePreferences()
const { data: types } = useAssetTypes(true)

const themes = [
  { label: 'Auto', value: 'system' as const, icon: 'pi pi-desktop' },
  { label: 'Light', value: 'light' as const, icon: 'pi pi-sun' },
  { label: 'Dark', value: 'dark' as const, icon: 'pi pi-moon' },
]
const densities = [
  { label: 'Comfortable', value: 'comfortable' as const },
  { label: 'Compact', value: 'compact' as const },
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
  <div class="settings">
    <SettingsGroup title="Appearance">
      <SettingsRow label="Theme">
        <SegmentedFilter v-model="store.prefs.theme" :options="themes" label="Theme" />
      </SettingsRow>
      <SettingsRow label="Table density" hint="Compact fits more rows on screen.">
        <SegmentedFilter v-model="store.prefs.density" :options="densities" label="Table density" />
      </SettingsRow>
    </SettingsGroup>

    <SettingsGroup title="Tables">
      <SettingsRow label="Default rows per page" hint="For every table where you haven't picked a size." for="pref-rows">
        <Select v-model="store.prefs.defaultPageSize" input-id="pref-rows" :options="PAGE_SIZES" class="narrow" />
      </SettingsRow>
      <SettingsRow v-if="ownSizes.length" label="Tables with their own size" hint="Remove one to use the default again." stacked>
        <Chip v-for="t in ownSizes" :key="t.key" :label="`${t.label} · ${t.size}`" removable @remove="useDefault(t.key)" />
        <Button label="Use the default everywhere" text size="small" @click="useDefaultEverywhere" />
      </SettingsRow>
    </SettingsGroup>

    <SettingsGroup title="Tabs" footer="Preferences are saved for your account on this device.">
      <SettingsRow label="Reopen my tabs when I sign in" hint="Off: only pinned tabs come back." for="pref-reopen">
        <ToggleSwitch v-model="store.prefs.reopenTabs" input-id="pref-reopen" />
      </SettingsRow>
    </SettingsGroup>
  </div>
</template>

<style scoped>
.narrow {
  width: 7rem;
}
</style>
