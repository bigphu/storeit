import { describe, expect, it } from 'vitest'
import { pageKeysBlocked } from './pageKeys'

describe('pageKeysBlocked', () => {
  const only = (sel: string) => (s: string) => (s.split(', ').includes(sel) ? {} : null)
  it('is blocked while the quick-edit drawer is open', () => {
    expect(pageKeysBlocked(only('.quick-edit'))).toBe(true)
  })
  it('is blocked by dialogs and popovers', () => {
    expect(pageKeysBlocked(only('.p-dialog-mask'))).toBe(true)
    expect(pageKeysBlocked(only('.p-popover'))).toBe(true)
  })
  it('is free when nothing overlays the page', () => {
    expect(pageKeysBlocked(() => null)).toBe(false)
  })
})
