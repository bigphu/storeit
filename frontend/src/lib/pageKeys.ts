// Phím tắt một chữ của một trang (J/K, E, /, N). Chỉ nghe khi trang đang hiện: trang
// ở tab nền vẫn được giữ sống (KeepAlive) nhưng không được nhận phím. Bỏ qua khi đang
// gõ trong ô nhập, khi giữ Ctrl/Alt/⌘, hay khi có hộp thoại/menu đang mở.
import { onActivated, onBeforeUnmount, onDeactivated, onMounted } from 'vue'

export function isTyping(e: KeyboardEvent): boolean {
  const t = e.target as HTMLElement | null
  return !!t && (/^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName) || t.isContentEditable)
}

export function usePageKeys(handler: (e: KeyboardEvent) => void) {
  function onKey(e: KeyboardEvent) {
    if (e.ctrlKey || e.metaKey || e.altKey || isTyping(e)) return
    if (document.querySelector('.p-dialog-mask, .p-popover, .p-contextmenu, .p-tieredmenu-overlay')) return
    handler(e)
  }
  let bound = false
  const add = () => {
    if (!bound) window.addEventListener('keydown', onKey)
    bound = true
  }
  const remove = () => {
    if (bound) window.removeEventListener('keydown', onKey)
    bound = false
  }
  onMounted(add)
  onActivated(add)
  onDeactivated(remove)
  onBeforeUnmount(remove)
}
