<script setup lang="ts">
import Card from 'primevue/card'
</script>

<template>
  <main class="public">
    <Card class="public-card">
      <template #title>StoreIt</template>
      <template #content>
        <RouterView />
      </template>
    </Card>
  </main>
</template>

<style scoped>
.public {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 1rem;
}
.public-card {
  width: min(100%, 26rem);
}
</style>
