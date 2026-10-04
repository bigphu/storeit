<script setup lang="ts">
import Tab from 'primevue/tab'
import TabList from 'primevue/tablist'
import TabPanel from 'primevue/tabpanel'
import TabPanels from 'primevue/tabpanels'
import Tabs from 'primevue/tabs'
import { useRouter } from 'vue-router'
import ChangePasswordForm from '../components/ChangePasswordForm.vue'
import PreferencesForm from '../components/PreferencesForm.vue'
import ProfileForm from '../components/ProfileForm.vue'

// Tài khoản của tôi; mục đang mở nằm trên URL (/account/:section)
defineProps<{ section: string }>()
const router = useRouter()

function open(section: string | number) {
  router.replace(`/account/${section}`)
}
</script>

<template>
  <section>
    <h1>Account settings</h1>
    <Tabs :value="section" @update:value="open">
      <TabList>
        <Tab value="profile">Profile</Tab>
        <Tab value="password">Password</Tab>
        <Tab value="preferences">Preferences</Tab>
      </TabList>
      <TabPanels>
        <TabPanel value="profile"><ProfileForm /></TabPanel>
        <TabPanel value="password"><ChangePasswordForm /></TabPanel>
        <TabPanel value="preferences"><PreferencesForm /></TabPanel>
      </TabPanels>
    </Tabs>
  </section>
</template>
