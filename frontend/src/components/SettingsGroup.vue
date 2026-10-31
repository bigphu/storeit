<script setup lang="ts">
// Nhóm cài đặt kiểu iOS Settings: tiêu đề nhỏ mờ phía trên, thẻ bo góc chứa các dòng
// (SettingsRow, ngăn bằng vạch mảnh), chú thích nhỏ phía dưới
defineProps<{ title?: string; footer?: string }>()
</script>

<template>
  <section class="settings-group">
    <h2 v-if="title" class="group-title">{{ title }}</h2>
    <div class="group-card"><slot /></div>
    <p v-if="footer || $slots.footer" class="group-footer">
      <slot name="footer">{{ footer }}</slot>
    </p>
  </section>
</template>

<style scoped>
.settings-group + .settings-group {
  margin-top: 1.6rem;
}
.group-title {
  margin: 0 0 0.45rem 1rem;
  font: 600 0.8rem var(--app-body);
  letter-spacing: 0;
  color: var(--p-text-muted-color);
}
.group-card {
  border: 1px solid var(--app-hair);
  border-radius: 14px;
  background: var(--app-panel);
  box-shadow: var(--app-card-shadow);
  overflow: hidden;
}
.group-footer {
  margin: 0.45rem 1rem 0;
  font-size: 0.8rem;
  color: var(--p-text-muted-color);
}
</style>
