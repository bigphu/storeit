// Thông báo dùng được ngoài component (QueryClient, session); App.vue hiện bằng Toast
type Severity = 'success' | 'info' | 'warn' | 'error'
export interface Notice {
  severity: Severity
  summary: string
}

type Listener = (n: Notice) => void
const listeners = new Set<Listener>()

export function onNotice(l: Listener): () => void {
  listeners.add(l)
  return () => listeners.delete(l)
}

function emit(severity: Severity, summary: string) {
  listeners.forEach((l) => l({ severity, summary }))
}

export const notify = {
  success: (s: string) => emit('success', s),
  info: (s: string) => emit('info', s),
  error: (s: string) => emit('error', s),
}
