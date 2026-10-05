<script setup lang="ts">
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import { ref } from 'vue'
import { identityApi } from '@/lib/api/client'
import { unwrap } from '@/lib/errors'
import { useFormErrors } from '@/lib/forms'

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
  <div class="form">
    <h2>Forgot password</h2>
    <Message v-if="sent" severity="success">
      If an account exists for {{ email }}, we've sent a link to reset the password.
    </Message>
    <form v-else class="form" @submit.prevent="submit">
      <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>
      <div class="field">
        <label for="email">Email</label>
        <InputText id="email" v-model="email" type="email" autocomplete="username" required />
        <small v-if="errors.fields.value.email" class="field-error">{{ errors.fields.value.email }}</small>
      </div>
      <Button type="submit" label="Send link" :loading="busy" />
    </form>
    <RouterLink to="/login">Back to sign in</RouterLink>
  </div>
</template>
