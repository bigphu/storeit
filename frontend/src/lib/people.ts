// Hiển thị người dùng: chữ viết tắt cho avatar và thời gian tương đối ("in 2 days").

export function initials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean)
  if (!words.length) return '?'
  const first = words[0][0]
  const last = words.length > 1 ? words[words.length - 1][0] : ''
  return (first + last).toUpperCase()
}

const rtf = new Intl.RelativeTimeFormat('en', { numeric: 'auto' })
const UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 365 * 24 * 3600],
  ['month', 30 * 24 * 3600],
  ['week', 7 * 24 * 3600],
  ['day', 24 * 3600],
  ['hour', 3600],
  ['minute', 60],
]

// relativeTime: "in 2 days", "yesterday", "5 minutes ago"; dưới một phút là "now"
export function relativeTime(iso: string, now: number = Date.now()): string {
  const secs = Math.round((new Date(iso).getTime() - now) / 1000)
  for (const [unit, size] of UNITS) {
    if (Math.abs(secs) >= size) return rtf.format(Math.round(secs / size), unit)
  }
  return 'now'
}

// inviteNote: dòng phụ dưới trạng thái Invited
export function inviteNote(expiresAt: string | undefined, now: number = Date.now()): string | undefined {
  if (!expiresAt) return undefined
  const when = relativeTime(expiresAt, now)
  return new Date(expiresAt).getTime() > now ? `Invite expires ${when}` : `Invite expired ${when}`
}
