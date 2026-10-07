// Thông báo dùng được ngoài component (QueryClient, session, runAction); App.vue vẽ bằng
// NoticeCard trong Toast. Thành công 4 giây, có Undo hay nút thì 8 giây, lỗi không tự đóng
export type Severity = 'success' | 'info' | 'warn' | 'error'

// Nút trong thông báo (vd "What’s inside" sau khi tải export dữ liệu)
export interface NoticeAction {
  label: string
  run: () => void
}

export interface Notice {
  id: number
  severity: Severity
  summary: string
  // lỗi: phần giải thích, hiện khi rê chuột hay focus
  detail?: string
  action?: NoticeAction
  undo?: () => void
  retry?: () => void
  // mili giây; 0 là không tự đóng
  life: number
}

export const LIFE = { plain: 4000, timed: 8000 } as const

type Listener = (n: Notice) => void
const listeners = new Set<Listener>()
let seq = 0

export function onNotice(l: Listener): () => void {
  listeners.add(l)
  return () => listeners.delete(l)
}

function emit(n: Omit<Notice, 'id'>): Notice {
  const full = { ...n, id: ++seq }
  listeners.forEach((l) => l(full))
  return full
}

export const notify = {
  success: (summary: string, o: { undo?: () => void; action?: NoticeAction } = {}) =>
    emit({ severity: 'success', summary, ...o, life: o.undo || o.action ? LIFE.timed : LIFE.plain }),
  info: (summary: string, o: { action?: NoticeAction } = {}) =>
    emit({ severity: 'info', summary, ...o, life: o.action ? LIFE.timed : LIFE.plain }),
  error: (summary: string, o: { detail?: string; retry?: () => void } = {}) =>
    emit({ severity: 'error', summary, ...o, life: 0 }),
}

// Ctrl/⌘ Z chạy Undo của thông báo mới nhất còn hiện. NoticeCard đăng ký khi hiện và
// xoá khi đóng
const undos: { id: number; run: () => void }[] = []

export function trackUndo(id: number, run: () => void) {
  undos.push({ id, run })
}

export function forgetUndo(id: number) {
  const i = undos.findIndex((u) => u.id === id)
  if (i >= 0) undos.splice(i, 1)
}

export function latestUndo(): (() => void) | undefined {
  return undos.at(-1)?.run
}

// isUndoShortcut: Ctrl/⌘ Z (không Shift, không Alt) và không đang gõ trong ô nhập
export function isUndoShortcut(e: KeyboardEvent): boolean {
  if (!(e.ctrlKey || e.metaKey) || e.shiftKey || e.altKey || e.key.toLowerCase() !== 'z') return false
  const t = e.target as { closest?: (s: string) => unknown } | null
  return !t?.closest?.('input, textarea, select, [contenteditable]:not([contenteditable="false"])')
}
