// Thông báo dùng được ngoài component (QueryClient, session); App.vue hiện bằng Toast
type Severity = 'success' | 'info' | 'warn' | 'error'
// Nút trong thông báo (vd "What’s inside" sau khi tải export dữ liệu)
export interface NoticeAction {
  label: string
  run: () => void
}
export interface Notice {
  severity: Severity
  summary: string
  action?: NoticeAction
}

type Listener = (n: Notice) => void
const listeners = new Set<Listener>()

export function onNotice(l: Listener): () => void {
  listeners.add(l)
  return () => listeners.delete(l)
}

function emit(severity: Severity, summary: string, action?: NoticeAction) {
  listeners.forEach((l) => l({ severity, summary, action }))
}

export const notify = {
  success: (s: string, action?: NoticeAction) => emit('success', s, action),
  info: (s: string, action?: NoticeAction) => emit('info', s, action),
  error: (s: string) => emit('error', s),
}
