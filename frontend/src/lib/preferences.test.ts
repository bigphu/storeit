import { describe, expect, it } from 'vitest'
import { defaultPrefs, isDark, pageSizeFor, PAGE_SIZES, pushRecent, tableLabel, withTableSize } from './preferences'

describe('page sizes', () => {
  const prefs = { ...defaultPrefs(), defaultPageSize: 25, tableSizes: { 'assets:L': 100 } }

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

describe('recent types', () => {
  it('moves the latest to the front, without duplicates, up to a limit', () => {
    expect(pushRecent(['a', 'b', 'c'], 'b')).toEqual(['b', 'a', 'c'])
    expect(pushRecent(['a', 'b', 'c', 'd', 'e'], 'f')).toEqual(['f', 'a', 'b', 'c', 'd'])
    expect(pushRecent([], 'a')).toEqual(['a'])
  })
})

describe('theme', () => {
  it('follows the device only for "system"', () => {
    expect(isDark('system', true)).toBe(true)
    expect(isDark('system', false)).toBe(false)
    expect(isDark('dark', false)).toBe(true)
    expect(isDark('light', true)).toBe(false)
  })
})

describe('tableLabel', () => {
  it('names tables for the preferences page', () => {
    const typeName = (id: string) => (id === 'L' ? 'Laptop' : undefined)
    expect(tableLabel('assets:all', typeName)).toBe('All assets')
    expect(tableLabel('assets:L', typeName)).toBe('Laptop')
    expect(tableLabel('assets:gone', typeName)).toBe('A removed asset type')
    expect(tableLabel('accounts', typeName)).toBe('Accounts')
  })
})
