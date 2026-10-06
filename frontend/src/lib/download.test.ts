import { describe, expect, it } from 'vitest'
import { fileNameFrom } from './download'

describe('fileNameFrom', () => {
  it('prefers the UTF-8 name', () => {
    expect(fileNameFrom(`attachment; filename*=utf-8''Ki%E1%BB%83m-k%C3%AA-2026-10-06.xlsx`, 'x.xlsx')).toBe('Kiểm-kê-2026-10-06.xlsx')
  })
  it('reads a quoted or plain name', () => {
    expect(fileNameFrom('attachment; filename="storeit-assets-2026-10-06.xlsx"', 'x.xlsx')).toBe('storeit-assets-2026-10-06.xlsx')
    expect(fileNameFrom('attachment; filename=a.xlsx', 'x.xlsx')).toBe('a.xlsx')
  })
  it('falls back without a header', () => {
    expect(fileNameFrom(null, 'x.xlsx')).toBe('x.xlsx')
  })
})
