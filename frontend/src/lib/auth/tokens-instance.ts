// Bộ token dùng chung của ứng dụng. session.ts gắn onSignedOut/onRefreshed.
import type { SessionResponse } from '@/lib/api/types'
import { createTokens } from './tokens'

type Listener = () => void
const signedOut = new Set<Listener>()
const refreshed = new Set<Listener>()

export const tokens = createTokens({
  // fetch thô, không qua authFetch: cookie refresh tự đi kèm vì cùng origin
  refresh: async () => {
    const res = await fetch('/api/v1/auth/refresh', { method: 'POST', credentials: 'same-origin' })
    if (res.status === 401) return null
    if (!res.ok) throw new Error(`refresh failed: HTTP ${res.status}`)
    return ((await res.json()) as SessionResponse).access_token
  },
  withLock: (fn) => (navigator.locks ? navigator.locks.request('storeit-refresh', fn) : fn()),
  onSignedOut: () => signedOut.forEach((l) => l()),
  onRefreshed: () => refreshed.forEach((l) => l()),
})

export function onTokensSignedOut(l: Listener) {
  signedOut.add(l)
}

export function onTokensRefreshed(l: Listener) {
  refreshed.add(l)
}
