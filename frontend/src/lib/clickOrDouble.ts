// Ô sửa được trong danh sách: nhấp một lần mở trang chi tiết, nhấp đúp sửa tại chỗ. Nhấp
// một lần đợi một chút xem có nhấp đúp không; Ctrl/⌘ và chuột giữa (mở tab mới) mở ngay
export interface ClickLike {
  ctrlKey: boolean
  metaKey: boolean
  button: number
}

export function clickOrDouble(delay = 220) {
  let timer: ReturnType<typeof setTimeout> | undefined
  return {
    click(e: ClickLike, open: () => void) {
      clearTimeout(timer)
      if (e.ctrlKey || e.metaKey || e.button === 1) {
        open()
        return
      }
      timer = setTimeout(open, delay)
    },
    double() {
      clearTimeout(timer)
    },
    cancel() {
      clearTimeout(timer)
    },
  }
}
