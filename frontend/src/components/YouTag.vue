<script setup lang="ts">
import Tag from 'primevue/tag'

// Nhãn "You" nhỏ cạnh tên của chính mình trong bảng: không theo bề rộng chung của tag
</script>

<template>
  <Tag value="You" severity="secondary" class="you-tag" />
</template>

<style scoped>
.you-tag {
  flex: none;
  min-width: 0;
  margin-left: 0.4rem;
  padding: 0.05rem 0.4rem;
  font-size: 0.7rem;
  vertical-align: middle;
}
</style>
