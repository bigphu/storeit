// Mở một vị trí trong app: bấm thường đi trong trang hiện tại, Ctrl/⌘ hay bấm giữa
// mở tab mới. Mọi chỗ "mở" (dòng bảng, menu chuột phải) đi qua đây, để sau này đổi
// "tab mới" thành tab trong app mà không sửa từng trang.
import type { RouteLocationRaw, Router } from 'vue-router'

type OpenEvent = { ctrlKey?: boolean; metaKey?: boolean; button?: number }

export function wantsNewTab(e: OpenEvent | undefined): boolean {
  return !!e && !!(e.ctrlKey || e.metaKey || e.button === 1)
}

export function openLocation(router: Router, to: RouteLocationRaw, e?: Event & OpenEvent, newTab = wantsNewTab(e)): void {
  if (newTab) {
    e?.preventDefault()
    window.open(router.resolve(to).href, '_blank', 'noopener')
    return
  }
  router.push(to)
}
