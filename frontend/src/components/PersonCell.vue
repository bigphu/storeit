<script setup lang="ts">
import Avatar from 'primevue/avatar'
import type { RouteLocationRaw } from 'vue-router'
import { initials } from '@/lib/people'
import YouTag from './YouTag.vue'

// Ô "người" trong bảng: avatar chữ viết tắt, tên (liên kết) cùng nhãn "You" trên một dòng,
// email bên dưới; muted cho tài khoản bị khoá. Ô chiếm hết bề ngang cột: slot #name (như ô
// sửa tại chỗ) thay cả tên lẫn nhãn "You", bút chì của nó nằm ở cuối cột
defineProps<{ name: string; email: string; to?: RouteLocationRaw; muted?: boolean; you?: boolean }>()
</script>

<template>
  <div class="person">
    <Avatar :label="initials(name)" shape="circle" class="person-avatar" :class="{ muted }" />
    <div class="person-text">
      <div class="person-line">
        <slot name="name">
          <RouterLink v-if="to" :to="to" class="person-name">{{ name }}</RouterLink>
          <span v-else class="person-name">{{ name }}</span>
          <YouTag v-if="you" />
        </slot>
      </div>
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
  flex: 1 1 auto;
  min-width: 0;
}
.person-line {
  display: flex;
  align-items: center;
  min-width: 0;
}
.person-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  font-weight: 600;
  color: var(--p-text-color);
  text-decoration: none;
}
.person-email {
  font-size: 0.82rem;
  color: var(--p-text-muted-color);
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
