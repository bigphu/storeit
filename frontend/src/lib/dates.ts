// Ngày (date, không giờ) đi lại với API dạng YYYY-MM-DD. Không dùng toISOString:
// nó đổi sang UTC và có thể lùi một ngày.

export function toDateString(d: Date | null | undefined): string | undefined {
  if (!d) return undefined
  const mm = String(d.getMonth() + 1).padStart(2, '0')
  const dd = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${mm}-${dd}`
}

export function fromDateString(s: string | null | undefined): Date | null {
  if (!s) return null
  const [y, m, d] = s.split('-').map(Number)
  return new Date(y, m - 1, d)
}

export function formatDate(s: string | null | undefined): string {
  const d = fromDateString(s)
  return d ? d.toLocaleDateString() : '—'
}

export function formatDateTime(s: string | null | undefined): string {
  return s ? new Date(s).toLocaleString() : '—'
}

// formatDay: ngày (theo giờ máy) của một thời điểm ISO, vd lần đăng nhập
export function formatDay(s: string | null | undefined): string {
  return s ? new Date(s).toLocaleDateString() : '—'
}
