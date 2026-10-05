<script setup lang="ts">
import ConfirmDialog from 'primevue/confirmdialog'
import Toast from 'primevue/toast'
import { useToast } from 'primevue/usetoast'
import { onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useSession } from '@/lib/auth/session'
import { usePreferences } from '@/lib/preferences'
import { onNotice } from '@/lib/notify'

const toast = useToast()
const off = onNotice((n) => toast.add({ severity: n.severity, summary: n.summary, life: 5000 }))
onUnmounted(off)

// Phiên hết hạn (refresh hỏng hoặc tab khác đăng xuất): về trang đăng nhập
const router = useRouter()
// nạp tuỳ chọn ngay từ đầu để theme áp cả trang đăng nhập
usePreferences()
useSession().setOnExpired(() => {
  const current = router.currentRoute.value
  if (!current.matched.some((r) => r.meta.public)) {
    router.push({ name: 'login', query: { redirect: current.fullPath } })
  }
})
</script>

<template>
  <RouterView />
  <Toast />
  <ConfirmDialog />
</template>
