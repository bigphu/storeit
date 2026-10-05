<script setup lang="ts">
import Button from 'primevue/button'
import Message from 'primevue/message'
import Password from 'primevue/password'
import { computed, ref } from 'vue'
import { identityApi } from '@/lib/api/client'
import { unwrap } from '@/lib/errors'
import { useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'

const errors = useFormErrors()
const current = ref('')
const password = ref('')
const confirm = ref('')
const busy = ref(false)
const mismatch = computed(() => confirm.value !== '' && confirm.value !== password.value)

async function submit() {
  if (mismatch.value) return
  busy.value = true
  errors.clear()
  try {
    await unwrap(
      identityApi.PUT('/auth/password', { body: { current_password: current.value, new_password: password.value } }),
    )
    current.value = password.value = confirm.value = ''
    notify.success('Password changed. Other devices have been signed out.')
  } catch (err) {
    errors.set(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <form class="form" @submit.prevent="submit">
    <Message severity="info">Changing your password signs out your other devices.</Message>
    <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>
    <div class="field">
      <label for="current">Current password</label>
      <Password
        v-model="current"
        input-id="current"
        :feedback="false"
        toggle-mask
        autocomplete="current-password"
        required
      />
      <small v-if="errors.fields.value.current_password" class="field-error">
        {{ errors.fields.value.current_password }}
      </small>
    </div>
    <div class="field">
      <label for="password">New password</label>
      <Password v-model="password" input-id="password" toggle-mask autocomplete="new-password" required />
      <small v-if="errors.fields.value.new_password" class="field-error">{{ errors.fields.value.new_password }}</small>
    </div>
    <div class="field">
      <label for="confirm">Confirm new password</label>
      <Password
        v-model="confirm"
        input-id="confirm"
        :feedback="false"
        toggle-mask
        autocomplete="new-password"
        required
      />
      <small v-if="mismatch" class="field-error">Passwords don't match</small>
    </div>
    <div class="actions">
      <Button type="submit" label="Change password" :loading="busy" :disabled="mismatch" />
    </div>
  </form>
</template>
