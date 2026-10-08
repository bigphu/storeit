<script setup lang="ts">
import { ICONS, type IconName } from './icons'

// Biểu tượng đặc; cỡ theo font-size (1em), màu theo currentColor
defineProps<{ name: IconName }>()
</script>

<template>
  <svg class="app-icon" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
    <path v-for="(d, i) in ICONS[name].paths" :key="i" :d="d" :fill-rule="ICONS[name].evenodd ? 'evenodd' : undefined" />
  </svg>
</template>

<style scoped>
.app-icon {
  display: block;
  width: 1em;
  height: 1em;
  flex: none;
}
</style>
