<script setup lang="ts">
import { useRouter } from 'vue-router'
import SegmentedFilter, { type SegmentOption } from '@/components/SegmentedFilter.vue'
import ChangePasswordForm from '../components/ChangePasswordForm.vue'
import PreferencesForm from '../components/PreferencesForm.vue'
import ProfileForm from '../components/ProfileForm.vue'

// Tài khoản của tôi; mục đang mở nằm trên URL (/account/:section)
defineProps<{ section: string }>()
const router = useRouter()

function open(section: string) {
  router.replace(`/account/${section}`)
}
const sections: SegmentOption<string>[] = [
  { value: 'profile', label: 'Profile', icon: 'pi pi-user' },
  { value: 'password', label: 'Password', icon: 'pi pi-lock' },
  { value: 'preferences', label: 'Preferences', icon: 'pi pi-sliders-h' },
]
</script>

<template>
  <section class="settings-page">
    <header class="settings-head">
      <h1>Settings</h1>
      <SegmentedFilter :model-value="section" :options="sections" label="Sections" @update:model-value="open" />
    </header>
    <ProfileForm v-if="section === 'profile'" />
    <ChangePasswordForm v-else-if="section === 'password'" />
    <PreferencesForm v-else-if="section === 'preferences'" />
  </section>
</template>

<style scoped>
/* cột cài đặt hẹp như Settings của iOS / macOS: tiêu đề và phân đoạn trên một hàng */
.settings-page {
  max-width: 42rem;
}
.settings-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 0.75rem;
  margin-bottom: 1.4rem;
}
</style>
