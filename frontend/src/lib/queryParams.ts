// Đọc tham số query của URL (một giá trị, hoặc số nguyên dương)

export function queryString(v: unknown): string | undefined {
  const s = Array.isArray(v) ? v[0] : v
  return typeof s === 'string' && s !== '' ? s : undefined
}

export function queryInt(v: unknown, fallback: number): number {
  const n = Number(queryString(v))
  return Number.isInteger(n) && n > 0 ? n : fallback
}
