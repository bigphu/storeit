// Ngăn kéo sửa nhanh của danh sách: mục đang sửa (item) giữ lại đến khi ngăn kéo trượt ra
// xong (SideDrawer báo closed), để hiệu ứng đóng chạy với nội dung còn nguyên; xong mới xoá
// mục và bản nháp (clear). Mở lại trước khi trượt xong thì không xoá
import { computed, type Ref, ref } from 'vue'

export function useQuickDrawer<T>(item: Ref<T | null>, clear: () => void) {
  const shown = ref(false)
  return {
    // v-model:visible của ngăn kéo: đóng chỉ ẩn, chưa xoá mục
    visible: computed({
      get: () => shown.value,
      set: (v: boolean) => {
        if (!v) shown.value = false
      },
    }),
    open(x: T) {
      clear()
      item.value = x
      shown.value = true
    },
    closed() {
      if (shown.value) return
      item.value = null
      clear()
    },
  }
}

export type QuickKeyAction = 'close' | 'save' | 'saveNext' | 'prev' | 'next' | null

// quickEditKey: phím nào làm gì trong ngăn sửa nhanh. J/K (khi không đang gõ) sang dòng
// trước/sau; Ctrl/⌘ S lưu; Ctrl/⌘ Enter lưu rồi sang dòng sau (không có gì đổi thì chỉ
// sang dòng sau), dùng được cả khi đang gõ; Esc đóng, trừ Esc trên hộp xác nhận
export function quickEditKey(
  e: { key: string; ctrlKey: boolean; metaKey: boolean; altKey: boolean },
  s: { dirty: boolean; busy: boolean; canPrev: boolean; canNext: boolean; typing: boolean; inDialog: boolean },
): QuickKeyAction {
  const mod = e.ctrlKey || e.metaKey
  if (e.key === 'Escape') return s.inDialog ? null : 'close'
  if (mod && e.key.toLowerCase() === 's') return s.dirty && !s.busy ? 'save' : null
  if (mod && e.key === 'Enter') {
    if (s.busy) return null
    if (s.dirty) return 'saveNext'
    return s.canNext ? 'next' : null
  }
  if (mod || e.altKey || s.typing) return null
  if (e.key === 'k' && s.canNext) return 'next'
  if (e.key === 'j' && s.canPrev) return 'prev'
  return null
}

// afterSaveNext: sau Ctrl Enter, chờ lần lưu chạy xong (busy lên rồi về false); hết thay
// đổi là lưu được thì sang dòng sau, còn thay đổi (lưu hỏng, huỷ xác nhận) thì ở lại
export function afterSaveNext(sawBusy: boolean, s: { busy: boolean; dirty: boolean; canNext: boolean }): 'next' | 'stay' | 'wait' {
  if (s.busy || !sawBusy) return 'wait'
  return !s.dirty && s.canNext ? 'next' : 'stay'
}

// saveNextTracker: Ctrl Enter đang chờ lần lưu chạy xong để sang dòng sau. Chờ theo busy của
// cha, không theo tiêu đề (đổi tên thì tiêu đề đổi ngay khi lưu xong, trước khi busy về
// false). settle() một nhịp sau khi phát save: cha chưa bật busy là không lưu (lỗi kiểm
// tra) nên thôi chờ, để lần lưu sau không kéo sang dòng khác
export function saveNextTracker() {
  let pending = false
  let sawBusy = false
  const cancel = () => {
    pending = false
    sawBusy = false
  }
  return {
    pending: () => pending,
    start() {
      pending = true
      sawBusy = false
    },
    cancel,
    settle(busy: boolean) {
      if (!pending) return
      if (busy) sawBusy = true
      else if (!sawBusy) cancel()
    },
    onBusy(s: { busy: boolean; dirty: boolean; canNext: boolean }): 'next' | null {
      if (!pending) return null
      if (s.busy) {
        sawBusy = true
        return null
      }
      const r = afterSaveNext(sawBusy, s)
      if (r === 'wait') return null
      cancel()
      return r === 'next' ? 'next' : null
    },
  }
}
