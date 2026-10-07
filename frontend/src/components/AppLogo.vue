<script setup lang="ts">
// Logo StoreIt: hộp đặc ba mặt màu brand (giống favicon) và tên. Tạm thời; khi có hình vẽ
// riêng thì thay ở đây
</script>

<template>
  <span class="app-logo">
    <svg class="mark" viewBox="0 0 24 24" aria-hidden="true">
      <g stroke-linejoin="round" stroke-width="1.2">
        <path d="M12 1.8 21.2 6.9 12 12 2.8 6.9Z" fill="#34d399" stroke="#34d399" />
        <path d="M2.8 6.9 12 12v10.2L2.8 17.1Z" fill="#10b981" stroke="#10b981" />
        <path d="M21.2 6.9V17.1L12 22.2V12Z" fill="#059669" stroke="#059669" />
      </g>
    </svg>
    StoreIt
  </span>
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
  width: 1.7rem;
  height: 1.7rem;
  flex: none;
}
</style>
