import { describe, expect, it } from 'vitest'
import { pageSizeFor, PAGE_SIZES, withTableSize } from './preferences'

describe('page sizes', () => {
  const prefs = { defaultPageSize: 25, tableSizes: { 'assets:L': 100 } }

  it('uses the table size, else the default', () => {
    expect(pageSizeFor(prefs, 'assets:L')).toBe(100)
    expect(pageSizeFor(prefs, 'assets:M')).toBe(25)
  })

  it('sets and clears a table size without touching others', () => {
    expect(withTableSize(prefs, 'accounts', 50).tableSizes).toEqual({ 'assets:L': 100, accounts: 50 })
    expect(withTableSize(prefs, 'assets:L', 'default').tableSizes).toEqual({})
    expect(prefs.tableSizes).toEqual({ 'assets:L': 100 })
  })

  it('offers sizes the API accepts (page_size 1..200)', () => {
    expect(PAGE_SIZES.every((n) => n >= 1 && n <= 200)).toBe(true)
  })
})
