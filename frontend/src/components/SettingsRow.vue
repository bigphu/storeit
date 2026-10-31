<script setup lang="ts">
// Một dòng trong SettingsGroup: nhãn (và gợi ý nhỏ) bên trái, điều khiển bên phải. stacked: điều
// khiển rộng (ô nhập, danh sách) xuống dưới nhãn. for: id của ô nhập để bấm nhãn là focus
defineProps<{ label: string; hint?: string; for?: string; stacked?: boolean }>()
</script>

<template>
  <div class="settings-row" :class="{ stacked }">
    <div class="row-text">
      <label v-if="$props.for" :for="$props.for" class="row-label">{{ label }}</label>
      <span v-else class="row-label">{{ label }}</span>
      <small v-if="hint" class="row-hint">{{ hint }}</small>
    </div>
    <div class="row-control"><slot /></div>
  </div>
</template>

<style scoped>
.settings-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  min-height: 3.1rem;
  padding: 0.65rem 1rem;
  box-sizing: border-box;
}
/* vạch ngăn thụt từ lề trái như danh sách của iOS */
.settings-row + .settings-row {
  box-shadow: inset 1rem 1px 0 var(--app-panel), inset 0 1px 0 var(--app-hair);
}
.settings-row.stacked {
  flex-direction: column;
  align-items: stretch;
  gap: 0.5rem;
}
.row-text {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  min-width: 0;
}
.row-label {
  font-weight: 500;
}
.row-hint {
  font-size: 0.8rem;
  color: var(--p-text-muted-color);
}
.row-control {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
  flex-wrap: wrap;
  min-width: 0;
}
.stacked .row-control {
  justify-content: flex-start;
}
@media (max-width: 640px) {
  .settings-row {
    flex-direction: column;
    align-items: stretch;
    gap: 0.5rem;
  }
  .row-control {
    justify-content: flex-start;
  }
}
</style>
