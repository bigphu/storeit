// Mượn tài sản (US-10): nhãn, kiểm form phiếu mượn, đọc yêu cầu thuộc tính thành chữ
import type { Attribute, Status, TicketState } from '@/lib/api/types'
import { fromDateString } from '@/lib/dates'
import type { Tone } from '@/lib/tones'
import { attrFilterLabel } from '@/features/assets/filterChips'
import type { AttrFilterRow } from '@/features/assets/listQuery'

// Hai status hệ thống chỉ việc mượn đặt (backend domain/borrowing.go): không chọn tay được
export const RESERVED_STATUS_ID = '00000000-0000-7000-8000-000000000205'
export const ON_LOAN_STATUS_ID = '00000000-0000-7000-8000-000000000206'

// isBorrowingStatus: Reserved hay On loan; tài sản mang status này không đổi status bằng tay
export function isBorrowingStatus(id: string | null | undefined): boolean {
  return id === RESERVED_STATUS_ID || id === ON_LOAN_STATUS_ID
}

// pickableStatuses: status chọn tay được (bỏ Reserved, On loan)
export function pickableStatuses<T extends { id: string }>(statuses: T[]): T[] {
  return statuses.filter((s) => !isBorrowingStatus(s.id))
}

// returnConditionOptions: tình trạng khi nhận lại tài sản; null là tốt (về Available mặc định),
// còn lại là status đang dùng không thuộc kind available hay retired và không do việc mượn đặt
export function returnConditionOptions(statuses: readonly Status[]): { label: string; value: string | null }[] {
  return [
    { label: 'Fine — back to Available', value: null },
    ...statuses
      .filter((s) => !s.archived_at && s.kind !== 'available' && s.kind !== 'retired' && !isBorrowingStatus(s.id))
      .map((s) => ({ label: s.name, value: s.id })),
  ]
}

// returnFormError: ngày trả từ ngày bắt đầu khoản mượn tới hôm nay (YYYY-MM-DD); null là hợp lệ
export function returnFormError(returnedOn: string, startedOn: string, today: string): string | null {
  if (!returnedOn) return 'Pick the return date.'
  if (returnedOn < startedOn) return `The loan started on ${shortDay(startedOn)}; pick that day or later.`
  if (returnedOn > today) return "That date hasn't come yet."
  return null
}

export const TICKET_STATE_LABEL: Record<TicketState, string> = {
  waiting: 'Waiting',
  approved: 'Ready for pickup',
  rejected: 'Rejected',
  cancelled: 'Cancelled',
  handed_over: 'Handed over',
}

// Màu tag của trạng thái phiếu (lib/tones): chờ cần chú ý, sẵn sàng nhận đang diễn ra, đã giao
// là xong việc, từ chối là hỏng, huỷ thì không hoạt động
export const TICKET_STATE_TONE: Record<TicketState, Tone> = {
  waiting: 'warn',
  approved: 'info',
  handed_over: 'success',
  rejected: 'danger',
  cancelled: 'secondary',
}

// overdueLabel: "Overdue 5 days"; chưa quá hạn thì rỗng
export function overdueLabel(days: number): string {
  if (days <= 0) return ''
  return `Overdue ${days} ${days === 1 ? 'day' : 'days'}`
}

const dayMonth = new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'short' })

// shortDay: ngày "YYYY-MM-DD" thành "7 Nov" (theo lịch, không đổi múi giờ)
export function shortDay(s: string | null | undefined): string {
  const d = fromDateString(s)
  return d ? dayMonth.format(d) : ''
}

// parseRequirement: "<key>:<op>:<value>", value có thể chứa ":"
export function parseRequirement(r: string): AttrFilterRow | null {
  const a = r.indexOf(':')
  const b = a < 0 ? -1 : r.indexOf(':', a + 1)
  if (a < 0 || b < 0) return null
  return { key: r.slice(0, a), op: r.slice(a + 1, b), value: r.slice(b + 1) }
}

type AttrLike = Pick<Attribute, 'key' | 'label' | 'data_type' | 'unit'> & { options: { id: string; label: string }[]; removed?: boolean }

// requirementLabels: yêu cầu thành chữ như chip bộ lọc ("RAM ≥ 16 GB"); điều kiện không còn khớp
// thuộc tính đang dùng nào của loại (thuộc tính đã gỡ) hiện nguyên văn
export function requirementLabels(reqs: string[], attrs: readonly AttrLike[]): string[] {
  const live = attrs.filter((a) => !a.removed)
  return reqs.map((r) => {
    const f = parseRequirement(r)
    return f && live.some((a) => a.key === f.key) ? attrFilterLabel(f, live) : r
  })
}

export interface TicketForm {
  typeId?: string
  neededFrom?: string // YYYY-MM-DD
  returnBy?: string
  note: string
}

const MAX_NOTE = 500

// ticketFormError: lỗi đầu tiên của form phiếu mượn, null là gửi được
export function ticketFormError(f: TicketForm, today: string): string | null {
  if (!f.typeId) return 'Pick what you need.'
  if (!f.neededFrom) return 'Pick the date you need it from.'
  if (f.neededFrom < today) return 'That date has passed.'
  if (f.returnBy && f.returnBy < f.neededFrom) return "Return by can't be before the date you need it from."
  if (f.note.trim().length > MAX_NOTE) return 'Keep the note under 500 characters.'
  return null
}

type ValueLike = { data_type: string; value: unknown }

// satisfies: giá trị thuộc tính của một tài sản có thoả một yêu cầu không, như bộ lọc của danh
// sách (chữ không phân biệt hoa thường, giá trị trống không bao giờ khớp). Hộp duyệt dùng để xếp
// tài sản khớp lên trước và gạch ô không khớp.
export function satisfies(f: AttrFilterRow, v: ValueLike | undefined): boolean {
  if (!v || v.value === null || v.value === undefined || v.value === '') return false
  const cmp = (a: number | string, b: number | string) => {
    switch (f.op) {
      case 'eq':
        return a === b
      case 'gt':
        return a > b
      case 'gte':
        return a >= b
      case 'lt':
        return a < b
      case 'lte':
        return a <= b
    }
    return false
  }
  switch (v.data_type) {
    case 'number':
      return cmp(Number(v.value), Number(f.value))
    case 'date':
      return cmp(String(v.value), f.value)
    case 'boolean':
      return f.op === 'eq' && String(v.value) === f.value
    case 'select':
      return f.op === 'in' ? f.value.split(',').includes(String(v.value)) : f.op === 'eq' && String(v.value) === f.value
    default: {
      const text = String(v.value).toLowerCase()
      const want = f.value.toLowerCase()
      return f.op === 'contains' ? text.includes(want) : f.op === 'eq' && text === want
    }
  }
}

// canSeeTicket: người xem mở được phiếu giữ chỗ / cho mượn một tài sản không: người quản lý, hay
// chính người mượn (API trả 404 cho người khác)
export function canSeeTicket(b: { account: { id: string } } | null | undefined, meId: string | undefined, canManage: boolean): boolean {
  return !!b && (canManage || b.account.id === meId)
}
