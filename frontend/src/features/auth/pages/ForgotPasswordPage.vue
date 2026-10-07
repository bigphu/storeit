<script setup lang="ts">
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { identityApi } from '@/lib/api/client'
import { unwrap } from '@/lib/errors'
import { useFormErrors } from '@/lib/forms'
import PublicResult from '../components/PublicResult.vue'

const router = useRouter()
const errors = useFormErrors()
const email = ref('')
const busy = ref(false)
const sent = ref(false)

async function submit() {
  busy.value = true
  errors.clear()
  try {
    // backend luôn trả 202, có tài khoản hay không
    await unwrap(identityApi.POST('/auth/password/forgot', { body: { email: email.value } }))
    sent.value = true
  } catch (err) {
    errors.set(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <PublicResult v-if="sent" icon="mail" tone="info" title="Check your email">
    If an account exists for <strong>{{ email }}</strong>, a link to reset the password is on its way.
    <template #actions>
      <Button label="Back to sign in" severity="secondary" outlined fluid @click="router.push('/login')" />
    </template>
  </PublicResult>
  <form v-else class="auth-form" @submit.prevent="submit">
    <div class="auth-head">
      <h2>Forgot password</h2>
      <p class="auth-hint">Enter your email and we'll send you a link to choose a new password.</p>
    </div>
    <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>
    <div class="field">
      <label for="email">Email</label>
      <InputText id="email" v-model="email" type="email" autocomplete="username" required fluid />
      <small v-if="errors.fields.value.email" class="field-error">{{ errors.fields.value.email }}</small>
    </div>
    <Button type="submit" label="Send link" :loading="busy" fluid />
    <p class="auth-foot"><RouterLink to="/login" class="auth-link">Back to sign in</RouterLink></p>
  </form>
</template>
