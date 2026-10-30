<script setup lang="ts">
import PageHeader from '@/components/PageHeader.vue'
import TicketTable from '../components/TicketTable.vue'

// Phiếu mượn của mọi người (người quản lý): duyệt, từ chối, giao
</script>

<template>
  <section>
    <PageHeader title="Tickets" subtitle="Requests to borrow assets. Approve one by reserving an asset, then hand it over." />
    <TicketTable :mine="false" />
  </section>
</template>
