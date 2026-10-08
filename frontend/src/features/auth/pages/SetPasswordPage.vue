<script setup lang="ts">
import Button from 'primevue/button'
import Message from 'primevue/message'
import Password from 'primevue/password'
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { identityApi } from '@/lib/api/client'
import { isApiError, unwrap } from '@/lib/errors'
import { useFormErrors } from '@/lib/forms'
import PublicResult from '../components/PublicResult.vue'

// Một trang cho cả link mời (accept-invite) và link đặt lại mật khẩu (reset-password)
const props = defineProps<{ mode: 'invite' | 'reset' }>()

const route = useRoute()
const router = useRouter()
const errors = useFormErrors()

const token = ref('')
const password = ref('')
const confirm = ref('')
const busy = ref(false)
const badLink = ref(false)

const title = computed(() => (props.mode === 'invite' ? 'Set up your account' : 'Reset password'))
const hint = computed(() =>
  props.mode === 'invite' ? 'Choose a password to finish setting up your account.' : 'Choose a new password for your account.',
)
const badLinkText = computed(() =>
  props.mode === 'invite'
    ? 'Invitation links work once and only for a limited time. Request a new link, or ask your administrator to resend the invitation.'
    : 'Reset links work once and only for a limited time. Request a new one to try again.',
)
const mismatch = computed(() => confirm.value !== '' && confirm.value !== password.value)

onMounted(() => {
  // Token nằm sau # để không lọt vào log server; đọc xong xoá khỏi thanh địa chỉ
  token.value = new URLSearchParams(route.hash.slice(1)).get('token') ?? ''
  if (route.hash) router.replace({ path: route.path, hash: '' })
  if (!token.value) badLink.value = true
})

async function submit() {
  if (mismatch.value) return
  busy.value = true
  errors.clear()
  try {
    await unwrap(identityApi.POST('/auth/password/set', { body: { token: token.value, new_password: password.value } }))
    await router.replace({ name: 'login', query: { notice: 'password-set' } })
  } catch (err) {
    if (isApiError(err, '/errors/invalid-password-token')) badLink.value = true
    else errors.set(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <PublicResult v-if="badLink" icon="alert" tone="warn" title="This link no longer works">
    {{ badLinkText }}
    <template #actions>
      <Button label="Request a new link" fluid @click="router.push('/forgot-password')" />
      <Button label="Back to sign in" severity="secondary" outlined fluid @click="router.push('/login')" />
    </template>
  </PublicResult>
  <form v-else class="auth-form" @submit.prevent="submit">
    <div class="auth-head">
      <h2>{{ title }}</h2>
      <p class="auth-hint">{{ hint }}</p>
    </div>
    <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>
    <div class="field">
      <label for="password">New password</label>
      <Password v-model="password" input-id="password" toggle-mask autocomplete="new-password" required fluid />
      <small v-if="errors.fields.value.new_password" class="field-error">{{ errors.fields.value.new_password }}</small>
    </div>
    <div class="field">
      <label for="confirm">Confirm password</label>
      <Password v-model="confirm" input-id="confirm" :feedback="false" toggle-mask autocomplete="new-password" required fluid />
      <small v-if="mismatch" class="field-error">Passwords don't match</small>
    </div>
    <Button type="submit" label="Set password" :loading="busy" :disabled="mismatch" fluid />
  </form>
</template>
