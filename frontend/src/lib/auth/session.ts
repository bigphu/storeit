// Phiên đăng nhập: tài khoản, vai trò, quyền (GET /me). Access token ở tokens-instance.
import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { identityApi } from '@/lib/api/client'
import type { Me } from '@/lib/api/types'
import { unwrap } from '@/lib/errors'
import { queryClient } from '@/lib/query'
import { onTokensRefreshed, onTokensSignedOut, tokens } from './tokens-instance'

// Báo các tab khác khi đăng nhập/đăng xuất
type SessionMessage = 'signed-in' | 'signed-out'
const channel = typeof BroadcastChannel !== 'undefined' ? new BroadcastChannel('storeit-session') : null

export const useSession = defineStore('session', () => {
  const me = ref<Me | null>(null)
  const ready = ref(false)
  let starting: Promise<void> | null = null
  // gọi khi phiên kết thúc ngoài ý muốn (refresh hỏng, tab khác đăng xuất); router gắn vào
  let onExpired: () => void = () => {}

  const signedIn = computed(() => me.value !== null)
  const permissions = computed(() => new Set(me.value?.permissions ?? []))

  function can(perm: string): boolean {
    return permissions.value.has(perm)
  }

  async function loadMe() {
    me.value = await unwrap(identityApi.GET('/me'))
  }

  // start: lần đầu mở app, thử refresh bằng cookie để biết còn phiên không
  function start(): Promise<void> {
    starting ??= (async () => {
      try {
        if (await tokens.refresh()) await loadMe()
      } catch {
        clearLocal()
      } finally {
        ready.value = true
      }
    })()
    return starting
  }

  async function login(email: string, password: string) {
    const s = await unwrap(identityApi.POST('/auth/login', { body: { email, password } }))
    tokens.set(s.access_token)
    await loadMe()
    channel?.postMessage('signed-in' satisfies SessionMessage)
  }

  async function logout() {
    try {
      await identityApi.POST('/auth/logout')
    } finally {
      clearLocal()
      channel?.postMessage('signed-out' satisfies SessionMessage)
    }
  }

  function clearLocal() {
    tokens.set(null)
    me.value = null
    queryClient.clear()
  }

  function setOnExpired(fn: () => void) {
    onExpired = fn
  }

  onTokensSignedOut(() => {
    clearLocal()
    onExpired()
  })
  // refresh nạp lại quyền: đổi vai trò có hiệu lực trong một vòng đời access token
  onTokensRefreshed(() => {
    if (me.value) loadMe().catch(() => {})
  })
  channel?.addEventListener('message', (e: MessageEvent<SessionMessage>) => {
    if (e.data === 'signed-out' && me.value) {
      clearLocal()
      onExpired()
    } else if (e.data === 'signed-in' && !me.value) {
      tokens
        .refresh()
        .then((t) => (t ? loadMe() : undefined))
        .catch(() => {})
    }
  })

  return { me, ready, signedIn, can, start, login, logout, loadMe, setOnExpired }
})
