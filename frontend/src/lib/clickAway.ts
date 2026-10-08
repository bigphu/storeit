// Bấm ra ngoài ngăn kéo sửa nhanh thì đóng nó, nhưng chỉ khi bấm trên trang (#app). Danh
// sách thả xuống, lịch, hộp xác nhận, toast của PrimeVue dựng ngoài #app: chọn trong đó
// không phải là bấm ra ngoài (trước đây chọn một lựa chọn làm ngăn kéo hỏi "Discard changes?")
type El = { closest?: (selector: string) => unknown }

export function isPageClickOutside(target: EventTarget | null, panel = '.quick-edit'): boolean {
  const el = target as El | null
  if (!el?.closest?.('#app')) return false
  return !el.closest(panel)
}
