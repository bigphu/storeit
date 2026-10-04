// Tab trong app: danh sách tab, tab đang mở, và đồng bộ với router (URL luôn là vị
// trí của tab đang mở). Lưu theo tài khoản trên máy này.
import { defineStore } from 'pinia'
import { computed, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useSession } from '@/lib/auth/session'
import { usePreferences } from '@/lib/preferences'
import { readJSON, writeJSON } from '@/lib/storage'
import { closeOthers as closeOthersOf, closeTab, initialTabs, openTab, pushBack, type Tab, togglePin as togglePinOf } from './tabList'

const HOME = '/assets'
let seq = 0
const newId = () => `t${Date.now().toString(36)}${(++seq).toString(36)}`

interface Saved {
  tabs: Tab[]
  activeId: string | null
  // đã ghim "All assets" lần đầu (initialTabs)
  seeded?: boolean
}

export const useTabs = defineStore('tabs', () => {
  const session = useSession()
  const prefs = usePreferences()
  // store được tạo trong component (layout), nên lấy được router của app
  const router = useRouter()

  const tabs = ref<Tab[]>([])
  const activeId = ref<string | null>(null)
  // tab của URL đang hiện: khi chuyển tab, activeId đổi ngay còn routeTabId đổi khi
  // URL của tab mới đã vào (trang được tạo cho đúng tab)
  const routeTabId = ref<string | null>(null)
  // tab có thay đổi chưa lưu (form): hiện chấm, hỏi lại khi đóng
  const dirty = reactive(new Set<string>())
  const flashId = ref<string | null>(null)
  // lần đồng bộ đầu sau khi đăng nhập: URL đang mở thành một tab (hoặc về tab có sẵn)
  let entered = false
  // goBack đang đưa tab về vị trí trước: lần đồng bộ tới không ghi vào lịch sử
  let goingBack = false

  const storageKey = computed(() => (session.me ? `storeit.tabs.${session.me.account.id}` : null))
  watch(
    storageKey,
    (key) => {
      entered = false
      dirty.clear()
      const saved = key ? readJSON<Partial<Saved>>('local', key, {}) : {}
      tabs.value = key ? initialTabs(saved.tabs, !!saved.seeded, prefs.prefs.reopenTabs, newId) : []
      activeId.value = tabs.value.some((t) => t.id === saved.activeId) ? saved.activeId! : (tabs.value[0]?.id ?? null)
    },
    { immediate: true },
  )
  watch(
    [tabs, activeId],
    () => {
      if (storageKey.value) writeJSON('local', storageKey.value, { tabs: tabs.value, activeId: activeId.value, seeded: true })
    },
    { deep: true },
  )

  const active = computed(() => tabs.value.find((t) => t.id === activeId.value) ?? null)
  const byId = (id: string) => tabs.value.find((t) => t.id === id)

  // sync: sau mỗi lần điều hướng, vị trí mới thuộc về tab đang mở
  function sync(path: string, title: string | undefined, icon: string | undefined) {
    syncPath(path, title, icon)
    routeTabId.value = activeId.value
  }

  function syncPath(path: string, title: string | undefined, icon: string | undefined) {
    if (!entered) {
      entered = true
      const same = tabs.value.find((t) => t.path === path)
      if (same) {
        activeId.value = same.id
        return
      }
      const tab: Tab = { id: newId(), path, pinned: false, title, icon }
      tabs.value = [...tabs.value, tab]
      activeId.value = tab.id
      return
    }
    const tab = active.value
    if (!tab) {
      const t: Tab = { id: newId(), path, pinned: false, title, icon }
      tabs.value = [...tabs.value, t]
      activeId.value = t.id
      return
    }
    if (tab.path !== path) {
      // đổi trang: tiêu đề tạm theo route, trang sẽ đặt tiêu đề cụ thể (useTabTitle).
      // Chỉ đổi query (lọc, trang) thì không tính là một bước trong lịch sử của tab
      const samePage = tab.path.split('?')[0] === path.split('?')[0]
      const back = goingBack || samePage ? tab.back : pushBack(tab.back, tab.path)
      Object.assign(tab, { path, icon, back, title: samePage ? tab.title : title })
    }
    goingBack = false
  }

  // goBack: về vị trí trước trong tab đang mở; false khi tab không có lịch sử
  async function goBack(): Promise<boolean> {
    const tab = active.value
    const prev = tab?.back?.at(-1)
    if (!tab || !prev) return false
    tab.back = tab.back!.slice(0, -1)
    goingBack = true
    await router.replace(prev)
    return true
  }

  function activate(id: string) {
    const tab = byId(id)
    if (!tab || id === activeId.value) return Promise.resolve()
    activeId.value = id
    return router.replace(tab.path)
  }

  function flash(id: string) {
    flashId.value = null
    setTimeout(() => (flashId.value = id))
    setTimeout(() => {
      if (flashId.value === id) flashId.value = null
    }, 900)
  }

  // open: tab mới (Ctrl-click, "Open in new tab"); trang đã có tab thì về tab đó
  function open(path: string, { background = true } = {}) {
    const meta = router.resolve(path).meta
    const r = openTab(tabs.value, activeId.value, path, { background }, newId, { title: meta.title, icon: meta.icon })
    tabs.value = r.tabs
    const target = r.opened ?? r.existing
    if (!target) return Promise.resolve()
    if (background) {
      flash(target)
      return Promise.resolve()
    }
    activeId.value = null // để activate điều hướng
    return activate(target)
  }

  function close(id: string) {
    const wasActive = id === activeId.value
    const r = closeTab(tabs.value, activeId.value, id)
    tabs.value = r.tabs
    dirty.delete(id)
    if (!tabs.value.length) {
      activeId.value = null
      return open(HOME, { background: false })
    }
    if (wasActive && r.activeId) {
      activeId.value = null
      return activate(r.activeId)
    }
    return Promise.resolve()
  }

  function closeOthers(id: string) {
    const keep = closeOthersOf(tabs.value, id)
    for (const t of tabs.value) if (!keep.includes(t)) dirty.delete(t.id)
    tabs.value = keep
    if (!keep.some((t) => t.id === activeId.value)) {
      activeId.value = null
      return activate(id)
    }
    return Promise.resolve()
  }

  function togglePin(id: string) {
    tabs.value = togglePinOf(tabs.value, id)
  }

  function rename(id: string, name: string) {
    const tab = byId(id)
    if (tab) tab.name = name.trim() || undefined
  }

  function duplicate(id: string) {
    const tab = byId(id)
    if (!tab) return Promise.resolve()
    const copy: Tab = { ...tab, id: newId(), pinned: false, name: undefined }
    const at = tabs.value.indexOf(tab) + 1
    tabs.value = [...tabs.value.slice(0, at), copy, ...tabs.value.slice(at)]
    activeId.value = null
    return activate(copy.id)
  }

  function setTitle(id: string, title: string) {
    const tab = byId(id)
    if (tab && tab.title !== title) tab.title = title
  }

  function setDirty(id: string, on: boolean) {
    if (on) dirty.add(id)
    else dirty.delete(id)
  }

  return {
    tabs,
    activeId,
    routeTabId,
    active,
    dirty,
    flashId,
    byId,
    sync,
    goBack,
    activate,
    open,
    close,
    closeOthers,
    togglePin,
    rename,
    duplicate,
    setTitle,
    setDirty,
  }
})
