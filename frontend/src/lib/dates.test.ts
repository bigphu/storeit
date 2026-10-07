import { describe, expect, it } from 'vitest'
import { formatDate, formatDateTime, formatDay } from './dates'

// Định dạng dùng Intl.DateTimeFormat dựng một lần: kết quả phải y như toLocale…String()
describe('date formatting', () => {
  it('formats a date like toLocaleDateString', () => {
    expect(formatDate('2026-03-25')).toBe(new Date(2026, 2, 25).toLocaleDateString())
    expect(formatDate(null)).toBe('—')
  })
  it('formats the day of an instant like toLocaleDateString', () => {
    const s = '2026-10-07T14:05:09Z'
    expect(formatDay(s)).toBe(new Date(s).toLocaleDateString())
  })
  it('formats an instant like toLocaleString', () => {
    const s = '2026-10-07T14:05:09Z'
    expect(formatDateTime(s)).toBe(new Date(s).toLocaleString())
    expect(formatDateTime(undefined)).toBe('—')
  })
})
