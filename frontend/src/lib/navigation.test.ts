import { describe, expect, it } from 'vitest'
import { wantsNewTab } from './navigation'

describe('wantsNewTab', () => {
  it('is true for Ctrl, Cmd and middle clicks', () => {
    expect(wantsNewTab({ ctrlKey: true } as MouseEvent)).toBe(true)
    expect(wantsNewTab({ metaKey: true } as MouseEvent)).toBe(true)
    expect(wantsNewTab({ button: 1 } as MouseEvent)).toBe(true)
    expect(wantsNewTab({ button: 0 } as MouseEvent)).toBe(false)
    expect(wantsNewTab(undefined)).toBe(false)
  })
})
