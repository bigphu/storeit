// Thay thế tài sản (US-13): kiểm form của hộp "Replace" và ngày hôm nay theo giờ máy
import { toDateString } from '@/lib/dates'

export interface ReplacementForm {
  replacementId?: string
  replacedOn?: string // YYYY-MM-DD
  reason: string
  // ngày mua (YYYY-MM-DD) của tài sản cũ và tài sản thay: ngày thay không được trước
  purchased?: Purchased[]
}

export interface Purchased {
  tag: string
  on?: string
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
  const early = f.purchased?.find((p) => p.on && f.replacedOn! < p.on)
  if (early) return `The date can't be before ${early.tag} was purchased (${early.on}).`
  const reason = f.reason.trim()
  if (!reason) return 'Give a reason.'
  if (reason.length > MAX_REASON) return 'Keep the reason under 500 characters.'
  return null
}

// earliestReplacementDay: ngày sớm nhất được chọn (ngày mua muộn nhất trong hai tài sản)
export function earliestReplacementDay(purchased: Purchased[]): string | undefined {
  return purchased.reduce<string | undefined>((max, p) => (p.on && (!max || p.on > max) ? p.on : max), undefined)
}

export interface ReplacementGroup<T> {
  label: string
  items: T[]
}

// replacementGroups: gợi ý cho hộp Replace, bỏ chính tài sản; tài sản cùng loại lên trước
// (thường thay bằng tài sản cùng loại), nhóm rỗng thì bỏ
export function replacementGroups<T extends { id: string; asset_type_id: string; asset_type_name: string }>(
  items: T[],
  self: { id: string; typeId: string },
): ReplacementGroup<T>[] {
  const others = items.filter((a) => a.id !== self.id)
  const same = others.filter((a) => a.asset_type_id === self.typeId)
  const rest = others.filter((a) => a.asset_type_id !== self.typeId)
  const groups: ReplacementGroup<T>[] = []
  if (same.length) groups.push({ label: same[0].asset_type_name, items: same })
  if (rest.length) groups.push({ label: 'Other types', items: rest })
  return groups
}
