<script setup lang="ts">
import Avatar from 'primevue/avatar'
import Tag from 'primevue/tag'
import type { RouteLocationRaw } from 'vue-router'
import { initials } from '@/lib/people'

// Ô "người" trong bảng: avatar chữ viết tắt, tên (liên kết), email; muted cho tài khoản bị khoá
defineProps<{ name: string; email: string; to?: RouteLocationRaw; muted?: boolean; you?: boolean }>()
</script>

<template>
  <div class="person">
    <Avatar :label="initials(name)" shape="circle" class="person-avatar" :class="{ muted }" />
    <div class="person-text">
      <slot name="name">
        <RouterLink v-if="to" :to="to" class="person-name">{{ name }}</RouterLink>
        <span v-else class="person-name">{{ name }}</span>
      </slot>
      <Tag v-if="you" value="You" severity="secondary" class="person-you" />
      <div class="person-email">{{ email }}</div>
    </div>
  </div>
</template>

<style scoped>
.person {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  min-width: 0;
}
.person-avatar {
  flex: none;
  background: var(--p-highlight-background);
  color: var(--p-highlight-color);
  font-weight: 700;
  font-size: 0.8rem;
}
.person-avatar.muted {
  background: var(--app-soft);
  color: var(--p-text-muted-color);
}
.person-text {
  min-width: 0;
}
.person-name {
  font-weight: 600;
  color: var(--p-text-color);
  text-decoration: none;
}
.person-you {
  /* nhãn nhỏ cạnh chữ: không theo bề rộng chung của tag */
  min-width: 0;
  margin-left: 0.4rem;
  font-size: 0.7rem;
}
.person-email {
  font-size: 0.82rem;
  color: var(--p-text-muted-color);
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
