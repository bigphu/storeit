// Lịch sử hoạt động của tài sản: câu tóm tắt mỗi dòng, thời gian tương đối, chữ dài cần thu
import type { AssetActivityEntry } from '@/lib/api/types'
import { formatDay } from '@/lib/dates'

export type ActivityEntry = AssetActivityEntry

// activitySummary: ai làm gì ("Lê Văn Hùng" · "edited 2 fields"); không có người là hệ thống
export function activitySummary(e: ActivityEntry): { who: string; text: string } {
  const who = e.actor?.name ?? 'System'
  switch (e.kind) {
    case 'created':
      return { who, text: 'created this asset' }
    case 'updated':
      return { who, text: e.changes.length === 1 ? `changed ${e.changes[0].label}` : `edited ${e.changes.length} fields` }
    case 'retired':
      return { who, text: e.reason ? `retired this asset: ${e.reason}` : 'retired this asset' }
    case 'restored':
      return { who, text: 'restored this asset' }
    case 'replaced':
      return { who, text: `replaced this asset with ${e.related?.tag ?? '?'}${e.reason ? `: ${e.reason}` : ''}` }
    case 'replaces':
      return { who, text: `recorded that this asset replaces ${e.related?.tag ?? '?'}` }
    case 'replacement_undone':
      return { who, text: `undid the replacement ${e.related?.role === 'new' ? 'of' : 'by'} ${e.related?.tag ?? '?'}` }
    default:
      return { who, text: `recorded ${e.type}` }
  }
}

const rtf = new Intl.RelativeTimeFormat('en', { numeric: 'auto' })
const MINUTE = 60_000
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

// relativeTime: "just now", "5 minutes ago", "yesterday"…; quá 30 ngày thì hiện ngày
export function relativeTime(iso: string, now: Date): string {
  const ms = now.getTime() - new Date(iso).getTime()
  if (ms < 45_000) return 'just now'
  if (ms < HOUR) return rtf.format(-Math.round(ms / MINUTE), 'minute')
  if (ms < DAY) return rtf.format(-Math.round(ms / HOUR), 'hour')
  if (ms <= 30 * DAY) return rtf.format(-Math.round(ms / DAY), 'day')
  return formatDay(iso)
}

// isLong: giá trị cần thu gọn (nút "Show more"): dài hơn 160 ký tự hay quá 3 dòng
export function isLong(text: string | null | undefined): boolean {
  if (!text) return false
  return text.length > 160 || text.split('\n').length > 3
}
