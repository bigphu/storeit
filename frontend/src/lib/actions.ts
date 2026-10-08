// Hành động thay đổi dữ liệu theo spec "Act now, offer Undo": chạy ngay, báo kết quả, cho
// Undo trong 8 giây. Mutation dùng qua đây phải có meta toast: false (lỗi do đây báo)
import { ApiError, describeError } from '@/lib/errors'
import { notify } from '@/lib/notify'

export interface UndoOptions<T> {
  // "Laptop archived."; hàm khi câu phụ thuộc kết quả
  done: string | ((result: T) => string)
  // đảo lại, nhận kết quả của lần chạy (vd version mới); không có thì không có nút Undo
  undo?: (result: T) => Promise<unknown>
  // "Laptop restored."
  undone?: string
  // nói điều đang đúng: "Couldn't restore Laptop. It is still archived."
  undoFailed?: string
}

export interface ActionOptions<T> extends UndoOptions<T> {
  run: () => Promise<T>
  // "Couldn't archive Laptop."; không có thì dùng câu của lỗi
  failed?: string
  // việc tiếp theo khi chạy được (rời trang, đóng hộp thoại); chạy cả khi thành công nhờ Retry
  after?: (result: T) => unknown
}

// announce: báo thành công (kèm Undo) cho việc đã chạy xong ở chỗ khác (form tự báo lỗi)
export function announce<T>(result: T, o: UndoOptions<T>): void {
  const done = typeof o.done === 'function' ? o.done(result) : o.done
  const undo = o.undo
  if (!undo) {
    notify.success(done)
    return
  }
  let used = false
  notify.success(done, {
    undo: () => {
      // bấm hai lần (nút và Ctrl+Z) chỉ đảo một lần
      if (used) return
      used = true
      undo(result).then(
        () => notify.success(o.undone ?? 'Undone.'),
        (err) => notify.error(o.undoFailed ?? "Couldn't undo that.", { detail: describeError(err) }),
      )
    },
  })
}

// worthRetrying: lỗi mạng, lỗi server (5xx) và 429 thì thử lại có thể được; lỗi khác của phía
// gọi (trùng tên, version cũ, dữ liệu sai) thử lại vẫn y như cũ nên không mời
export function worthRetrying(err: unknown): boolean {
  return !(err instanceof ApiError) || err.status >= 500 || err.status === 429
}

export async function runAction<T>(o: ActionOptions<T>): Promise<boolean> {
  let result: T
  try {
    result = await o.run()
  } catch (err) {
    const why = describeError(err)
    // Retry chạy một lần (bấm hai lần lúc thông báo đang mờ đi không chạy hai lần)
    let retried = false
    const retry = () => {
      if (retried) return
      retried = true
      void runAction(o)
    }
    notify.error(o.failed ?? why, { detail: o.failed ? why : undefined, retry: worthRetrying(err) ? retry : undefined })
    return false
  }
  announce(result, o)
  await o.after?.(result)
  return true
}
