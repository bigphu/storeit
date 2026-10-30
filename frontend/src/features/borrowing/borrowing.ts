// Mượn tài sản (US-10): nhãn, kiểm form phiếu mượn, đọc yêu cầu thuộc tính thành chữ
import type { Attribute, TicketState } from '@/lib/api/types'
import { fromDateString } from '@/lib/dates'
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

export const TICKET_STATE_LABEL: Record<TicketState, string> = {
  waiting: 'Waiting',
  approved: 'Ready for pickup',
  rejected: 'Rejected',
  cancelled: 'Cancelled',
  handed_over: 'Handed over',
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

type AttrLike = Pick<Attribute, 'key' | 'label' | 'data_type' | 'unit'> & { options: { id: string; label: string }[] }

// requirementLabels: yêu cầu thành chữ như chip bộ lọc ("RAM ≥ 16 GB"); điều kiện không còn khớp
// thuộc tính nào của loại (thuộc tính đã gỡ) hiện nguyên văn
export function requirementLabels(reqs: string[], attrs: readonly AttrLike[]): string[] {
  return reqs.map((r) => {
    const f = parseRequirement(r)
    return f && attrs.some((a) => a.key === f.key) ? attrFilterLabel(f, attrs) : r
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
