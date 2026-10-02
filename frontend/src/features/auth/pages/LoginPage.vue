<script setup lang="ts">
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Password from 'primevue/password'
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { redirectTarget } from '@/app/router'
import { useSession } from '@/lib/auth/session'
import { useFormErrors } from '@/lib/forms'

const session = useSession()
const route = useRoute()
const router = useRouter()
const errors = useFormErrors()

const email = ref('')
const password = ref('')
const busy = ref(false)
// thông báo theo mã (không hiện chữ tuỳ ý từ URL)
const notices: Record<string, string> = { 'password-set': 'Password set. Please sign in.' }
const notice = notices[String(route.query.notice)] ?? ''

async function submit() {
  busy.value = true
  errors.clear()
  try {
    await session.login(email.value, password.value)
    await router.replace(redirectTarget(route.query.redirect))
  } catch (err) {
    errors.set(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <form class="form" @submit.prevent="submit">
    <h2>Sign in</h2>
    <Message v-if="notice" severity="success">{{ notice }}</Message>
    <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>
    <div class="field">
      <label for="email">Email</label>
      <InputText id="email" v-model="email" type="email" autocomplete="username" required />
      <small v-if="errors.fields.value.email" class="field-error">{{ errors.fields.value.email }}</small>
    </div>
    <div class="field">
      <label for="password">Password</label>
      <Password
        v-model="password"
        input-id="password"
        :feedback="false"
        toggle-mask
        autocomplete="current-password"
        required
      />
    </div>
    <Button type="submit" label="Sign in" :loading="busy" />
    <RouterLink to="/forgot-password">Forgot password?</RouterLink>
  </form>
</template>
