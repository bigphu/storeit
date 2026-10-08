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

// Bộ định dạng dựng một lần, lần đầu cần: toLocale…String() dựng bộ mới mỗi lần gọi, tốn
// khi bảng có hàng trăm ô ngày. Tuỳ chọn trùng mặc định của toLocaleDateString/toLocaleString
let dayFmt: Intl.DateTimeFormat | undefined
let dateTimeFmt: Intl.DateTimeFormat | undefined
const day = (d: Date) => (dayFmt ??= new Intl.DateTimeFormat(undefined, { year: 'numeric', month: 'numeric', day: 'numeric' })).format(d)
const dateTime = (d: Date) =>
  (dateTimeFmt ??= new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: 'numeric',
    day: 'numeric',
    hour: 'numeric',
    minute: 'numeric',
    second: 'numeric',
  })).format(d)

export function formatDate(s: string | null | undefined): string {
  const d = fromDateString(s)
  return d ? day(d) : '—'
}

export function formatDateTime(s: string | null | undefined): string {
  return s ? dateTime(new Date(s)) : '—'
}

// formatDay: ngày (theo giờ máy) của một thời điểm ISO, vd lần đăng nhập
export function formatDay(s: string | null | undefined): string {
  return s ? day(new Date(s)) : '—'
}
