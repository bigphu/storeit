// Mở một vị trí trong app: bấm thường đi trong trang hiện tại, Ctrl/⌘ hay bấm giữa
// mở tab mới. Mọi chỗ "mở" (dòng bảng, menu chuột phải) đi qua đây; layout đăng ký
// cách mở tab mới (tab trong app), mặc định là tab của trình duyệt.
import type { RouteLocationRaw, Router } from 'vue-router'

type OpenEvent = { ctrlKey?: boolean; metaKey?: boolean; button?: number }

export function wantsNewTab(e: OpenEvent | undefined): boolean {
  return !!e && !!(e.ctrlKey || e.metaKey || e.button === 1)
}

let openInNewTab: (path: string) => void = (path) => window.open(path, '_blank', 'noopener')

export function setNewTabHandler(fn: (path: string) => void) {
  openInNewTab = fn
}

export function openLocation(router: Router, to: RouteLocationRaw, e?: Event & OpenEvent, newTab = wantsNewTab(e)): void {
  if (newTab) {
    e?.preventDefault()
    openInNewTab(router.resolve(to).fullPath)
    return
  }
  router.push(to)
}
