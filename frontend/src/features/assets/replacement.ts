// Thay thế tài sản (US-13): kiểm form của hộp "Replace" và ngày hôm nay theo giờ máy
import { toDateString } from '@/lib/dates'

export interface ReplacementForm {
  replacementId?: string
  replacedOn?: string // YYYY-MM-DD
  reason: string
}

const MAX_REASON = 500

// todayLocal: ngày hôm nay (YYYY-MM-DD) theo giờ của máy người dùng
export function todayLocal(now: Date): string {
  return toDateString(now) ?? ''
}

// replacementFormError: lỗi đầu tiên của form theo thứ tự các ô, null là gửi được
export function replacementFormError(f: ReplacementForm, today: string): string | null {
  if (!f.replacementId) return 'Pick the replacement asset.'
  if (!f.replacedOn) return 'Pick the date.'
  if (f.replacedOn > today) return "The date can't be in the future."
  const reason = f.reason.trim()
  if (!reason) return 'Give a reason.'
  if (reason.length > MAX_REASON) return 'Keep the reason under 500 characters.'
  return null
}
