// Cho trang biết nó thuộc tab nào. Trang được giữ sống khi chuyển tab (KeepAlive),
// nên trang ở tab nền không được phản ứng với URL của tab khác.
import { onActivated, onDeactivated, onScopeDispose, shallowRef, watch, watchEffect } from 'vue'
import { type LocationQuery, useRoute } from 'vue-router'
import { useTabs } from './useTabs'

// useTabId: tab của trang này (trang được tạo cho tab của URL đang hiện)
export function useTabId(): string | null {
  return useTabs().routeTabId
}

// useTabQuery: query của URL, chỉ cập nhật khi trang đang hiện trong tab của nó.
// Rời sang trang khác trong cùng tab hay sang tab khác thì giữ query cũ.
export function useTabQuery() {
  const route = useRoute()
  const tabs = useTabs()
  const tabId = tabs.routeTabId
  const record = route.matched.at(-1)?.path
  const query = shallowRef<LocationQuery>(route.query)
  watch(
    () => route.query,
    (q) => {
      if (tabs.activeId === tabId && route.matched.at(-1)?.path === record) query.value = q
    },
  )
  return query
}

// trang đang hiện (không bị KeepAlive cất đi)
function useShown() {
  const shown = shallowRef(true)
  onActivated(() => (shown.value = true))
  onDeactivated(() => (shown.value = false))
  return shown
}

// useTabTitle: tiêu đề của tab theo dữ liệu của trang ("LAP-0042", "Laptop · RAM ≥ 16")
export function useTabTitle(title: () => string | undefined) {
  const tabs = useTabs()
  const tabId = tabs.routeTabId
  const shown = useShown()
  // Chỉ theo tiêu đề của trang và việc trang đang hiện. Không dùng watchEffect: setTitle
  // đọc tiêu đề hiện tại của tab, nên khi rời trang (Back) sync đổi tiêu đề tab theo route
  // mới thì effect chạy lại trước khi KeepAlive cất trang đi và ghi đè tiêu đề cũ lên tab
  watch(
    [title, shown],
    ([t, isShown]) => {
      if (tabId && t && isShown) tabs.setTitle(tabId, t)
    },
    { immediate: true },
  )
}

// useTabDirty: chấm "chưa lưu" trên tab, hỏi lại khi đóng tab. Chuyển sang tab khác
// thì chấm vẫn còn; rời form trong cùng tab thì bỏ chấm (bản nháp vẫn được giữ)
export function useTabDirty(dirty: () => boolean) {
  const tabs = useTabs()
  const tabId = tabs.routeTabId
  const onForm = shallowRef(true)
  onActivated(() => (onForm.value = true))
  onDeactivated(() => {
    if (tabs.activeId === tabId) onForm.value = false
  })
  watchEffect(() => {
    if (tabId) tabs.setDirty(tabId, onForm.value && dirty())
  })
  onScopeDispose(() => {
    if (tabId) tabs.setDirty(tabId, false)
  })
}
