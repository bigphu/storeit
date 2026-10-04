// Danh sách tab trong app (thuần, không phụ thuộc Vue): mỗi tab giữ một vị trí
// (fullPath của router). Tab ghim luôn đứng đầu.

export interface Tab {
  id: string
  path: string
  pinned: boolean
  // tên người dùng đặt (đổi tên tab); không có thì dùng tiêu đề của trang
  name?: string
  // tiêu đề và biểu tượng do trang đặt (useTabTitle) hoặc theo route
  title?: string
  icon?: string
  // các vị trí trước đó trong tab, mới nhất cuối (quay lại trong tab, không lẫn tab khác)
  back?: string[]
}

const BACK_LIMIT = 20

// pushBack: thêm vị trí vừa rời vào lịch sử của tab
export function pushBack(back: string[] | undefined, path: string): string[] {
  return [...(back ?? []), path].slice(-BACK_LIMIT)
}

// Danh sách (tài sản, tài khoản, vai trò) mở được nhiều tab với bộ lọc khác nhau;
// trang khác (một tài sản, cài đặt) chỉ một tab: mở lại thì về tab có sẵn
const LIST_PATHS = [/^\/assets$/, /^\/types\/[^/]+\/assets$/, /^\/accounts$/, /^\/roles$/]

export function dedupeKey(path: string): string | null {
  const pathname = path.split(/[?#]/)[0]
  return LIST_PATHS.some((re) => re.test(pathname)) ? null : pathname
}

const pinnedCount = (tabs: Tab[]) => tabs.filter((t) => t.pinned).length

export interface OpenResult {
  tabs: Tab[]
  activeId: string | null
  // tab vừa tạo, hoặc tab có sẵn đang hiện trang đó
  opened?: string
  existing?: string
}

// openTab: tab mới ngay sau tab đang mở (không chen vào nhóm tab ghim)
export function openTab(
  tabs: Tab[],
  activeId: string | null,
  path: string,
  { background }: { background: boolean },
  newId: () => string,
): OpenResult {
  const key = dedupeKey(path)
  const existing = key ? tabs.find((t) => dedupeKey(t.path) === key) : undefined
  if (existing) return { tabs, activeId: background ? activeId : existing.id, existing: existing.id }
  const tab: Tab = { id: newId(), path, pinned: false }
  const at = Math.max(tabs.findIndex((t) => t.id === activeId) + 1, pinnedCount(tabs))
  const next = [...tabs.slice(0, at), tab, ...tabs.slice(at)]
  return { tabs: next, activeId: background ? activeId : tab.id, opened: tab.id }
}

// closeTab: đóng tab đang mở thì sang tab bên phải, hết thì bên trái
export function closeTab(tabs: Tab[], activeId: string | null, id: string): { tabs: Tab[]; activeId: string | null } {
  const i = tabs.findIndex((t) => t.id === id)
  if (i < 0) return { tabs, activeId }
  const next = tabs.filter((t) => t.id !== id)
  if (activeId !== id) return { tabs: next, activeId }
  return { tabs: next, activeId: (next[i] ?? next[i - 1])?.id ?? null }
}

export function closeOthers(tabs: Tab[], id: string): Tab[] {
  return tabs.filter((t) => t.id === id || t.pinned)
}

// togglePin: ghim thì về cuối nhóm ghim; bỏ ghim thì đứng đầu nhóm chưa ghim
export function togglePin(tabs: Tab[], id: string): Tab[] {
  const tab = tabs.find((t) => t.id === id)
  if (!tab) return tabs
  const changed = { ...tab, pinned: !tab.pinned }
  const rest = tabs.filter((t) => t.id !== id)
  // cả hai trường hợp tab đều nằm ở ranh giới giữa nhóm ghim và nhóm chưa ghim
  return [...rest.filter((t) => t.pinned), changed, ...rest.filter((t) => !t.pinned)]
}

// restoreTabs: tab đã lưu khi đăng nhập lại; tắt "mở lại tab" thì chỉ giữ tab ghim
export function restoreTabs(saved: unknown, reopenAll: boolean): Tab[] {
  if (!Array.isArray(saved)) return []
  return saved
    .filter((t): t is Tab => !!t && typeof t.id === 'string' && typeof t.path === 'string' && t.path.startsWith('/'))
    .map((t) => ({ ...t, pinned: !!t.pinned }))
    .filter((t) => reopenAll || t.pinned)
}

// initialTabs: tab khi đăng nhập. Một lần duy nhất (seeded chưa đặt): tab "All assets"
// được ghim (tab /assets có sẵn thì ghim nó, không thì thêm). Sau đó người dùng bỏ ghim
// hay đóng nó thì giữ như vậy.
export function initialTabs(saved: unknown, seeded: boolean, reopenAll: boolean, newId: () => string): Tab[] {
  const tabs = restoreTabs(saved, reopenAll)
  if (seeded) return tabs
  const all = tabs.find((t) => t.path === '/assets')
  const pinnedAll: Tab = all
    ? { ...all, pinned: true }
    : { id: newId(), path: '/assets', pinned: true, title: 'All assets', icon: 'pi pi-th-large' }
  return [pinnedAll, ...tabs.filter((t) => t !== all)]
}
