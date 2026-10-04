<script setup lang="ts">
import Breadcrumb from 'primevue/breadcrumb'
import type { RouteLocationRaw } from 'vue-router'

// Breadcrumb của PrimeVue với liên kết router; mục cuối (trang hiện tại) không có `to`
export interface Crumb {
  label: string
  to?: RouteLocationRaw
}

defineProps<{ items: Crumb[] }>()
</script>

<template>
  <Breadcrumb :model="items" class="app-breadcrumb">
    <template #item="{ item, props }">
      <RouterLink v-if="item.to" v-slot="{ href, navigate }" :to="item.to" custom>
        <a :href="href" v-bind="props.action" @click="navigate">{{ item.label }}</a>
      </RouterLink>
      <span v-else v-bind="props.action" aria-current="page">{{ item.label }}</span>
    </template>
  </Breadcrumb>
</template>

<style scoped>
.app-breadcrumb {
  padding: 0;
  margin-bottom: 0.75rem;
  background: transparent;
}
</style>
