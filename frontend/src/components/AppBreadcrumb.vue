<script setup lang="ts">
import Breadcrumb from 'primevue/breadcrumb'
import Button from 'primevue/button'
import { computed } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
import { useTabs } from '@/app/tabs/useTabs'

// Breadcrumb của PrimeVue với liên kết router; mục cuối (trang hiện tại) không có `to`.
// Nút ← quay lại trang trước trong tab này (lịch sử riêng của tab)
export interface Crumb {
  label: string
  to?: RouteLocationRaw
}

defineProps<{ items: Crumb[] }>()

const tabs = useTabs()
const canGoBack = computed(() => !!tabs.active?.back?.length)
</script>

<template>
  <div class="crumbs-row">
    <Button
      icon="pi pi-arrow-left"
      text
      rounded
      size="small"
      severity="secondary"
      class="back-btn"
      :disabled="!canGoBack"
      aria-label="Back in this tab"
      title="Back in this tab"
      @click="tabs.goBack()"
    />
    <Breadcrumb :model="items" class="app-breadcrumb">
      <template #item="{ item, props }">
        <RouterLink v-if="item.to" v-slot="{ href, navigate }" :to="item.to" custom>
          <a :href="href" v-bind="props.action" @click="navigate">{{ item.label }}</a>
        </RouterLink>
        <span v-else v-bind="props.action" aria-current="page">{{ item.label }}</span>
      </template>
    </Breadcrumb>
  </div>
</template>

<style scoped>
.crumbs-row {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  margin-bottom: 0.75rem;
}
.back-btn {
  width: 1.9rem;
  height: 1.9rem;
}
.app-breadcrumb {
  padding: 0;
  background: transparent;
}
</style>
