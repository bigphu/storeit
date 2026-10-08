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
