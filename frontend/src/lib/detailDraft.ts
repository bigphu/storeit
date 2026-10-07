// Bản nháp của trang chi tiết (và ngăn kéo sửa nhanh): thay đổi chưa lưu theo từng tab.
// Overview: trường → giá trị; danh sách tick (quyền, role): khoá → có/không. Thay đổi trùng
// giá trị đã lưu thì bỏ, nên số thay đổi luôn đúng. Thuần, để thử được; trang bọc bằng reactive()
export type DraftValue = string | number | boolean | null
export type Draft = Record<string, Record<string, DraftValue>>

export function emptyDraft(): Draft {
  return {}
}

export function setEdit(d: Draft, tab: string, key: string, value: DraftValue, saved: DraftValue) {
  const t = (d[tab] ??= {})
  if (value === saved) delete t[key]
  else t[key] = value
}

export function valueOf(d: Draft, tab: string, key: string, saved: DraftValue): DraftValue {
  const t = d[tab]
  return t && key in t ? t[key] : saved
}

export const changeCount = (d: Draft, tab: string) => Object.keys(d[tab] ?? {}).length
export const isDirty = (d: Draft) => Object.values(d).some((t) => Object.keys(t).length > 0)
export const changesOf = (d: Draft, tab: string): Record<string, DraftValue> => ({ ...(d[tab] ?? {}) })

// discardTab: bỏ thay đổi của tab, trả lại để Undo đặt lại
export function discardTab(d: Draft, tab: string): Record<string, DraftValue> {
  const removed = changesOf(d, tab)
  delete d[tab]
  return removed
}

// restoreTab: Undo của Discard; gộp vào thay đổi đang có (đã sửa thêm sau Discard thì giữ)
export function restoreTab(d: Draft, tab: string, edits: Record<string, DraftValue>) {
  d[tab] = { ...edits, ...(d[tab] ?? {}) }
}

// clearTab: sau khi lưu xong
export function clearTab(d: Draft, tab: string) {
  delete d[tab]
}

// previousOf: giá trị đã lưu của các khoá vừa đổi; Undo của Save gửi lại những giá trị này
export function previousOf(saved: Record<string, DraftValue>, changes: Record<string, DraftValue>): Record<string, DraftValue> {
  return Object.fromEntries(Object.keys(changes).map((k) => [k, saved[k] ?? null]))
}

// setList: danh sách tick mới so với danh sách đã lưu, ghi từng khoá
export function setList(d: Draft, tab: string, saved: string[], next: string[]) {
  for (const k of new Set([...saved, ...next])) setEdit(d, tab, k, next.includes(k), saved.includes(k))
}

// listOf: danh sách tick hiện tại (đã lưu + bản nháp), theo thứ tự của all
export function listOf(d: Draft, tab: string, saved: string[], all: string[]): string[] {
  return all.filter((k) => valueOf(d, tab, k, saved.includes(k)) === true)
}
