<script setup lang="ts">
import Button from 'primevue/button'
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import KeyHint from '@/components/KeyHint.vue'
import PageHeader from '@/components/PageHeader.vue'
import { usePageKeys } from '@/lib/pageKeys'
import NewTicketDialog from '../components/NewTicketDialog.vue'
import TicketTable from '../components/TicketTable.vue'

// Phiếu mượn của tôi: gửi phiếu mới (phím N), theo dõi, huỷ
const creating = ref(false)
const router = useRouter()
usePageKeys((e) => {
  if (e.key === 'n') creating.value = true
})
</script>

<template>
  <section>
    <PageHeader title="My tickets" subtitle="Ask to borrow an asset and follow your requests.">
      <Button aria-label="New ticket (N)" @click="creating = true">
        <i class="pi pi-plus" aria-hidden="true" />
        <span>New ticket</span>
        <KeyHint keys="N" />
      </Button>
    </PageHeader>
    <TicketTable mine />
    <NewTicketDialog v-model:visible="creating" @created="(t) => router.push(`/borrowing/tickets/${t.id}`)" />
  </section>
</template>
