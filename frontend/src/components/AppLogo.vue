<script setup lang="ts">
// Logo StoreIt: ô vuông màu brand có chữ S và tên. Chưa có hình vẽ riêng; khi có thì thay ở đây
</script>

<template>
  <span class="app-logo"><span class="mark" aria-hidden="true">S</span>StoreIt</span>
</template>

<style scoped>
.app-logo {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font: 800 1.45rem var(--app-display);
  letter-spacing: -0.02em;
}
.mark {
  display: grid;
  place-items: center;
  width: 1.6rem;
  height: 1.6rem;
  border-radius: 7px;
  background: var(--app-brand);
  color: var(--app-on-color);
  font-size: 0.9rem;
}
</style>
