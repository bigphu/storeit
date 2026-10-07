import { describe, expect, it } from 'vitest'
import { isPageClickOutside } from './clickAway'

// phần tử giả: closest trả về khác null khi phần tử nằm trong các selector đã cho
const at = (...inside: string[]) => ({ closest: (s: string) => (inside.includes(s) ? {} : null) })

describe('isPageClickOutside', () => {
  it('counts a click on the page behind the drawer', () => {
    expect(isPageClickOutside(at('#app') as unknown as EventTarget)).toBe(true)
  })
  it('ignores a click in an overlay outside the page, like a dropdown option or a confirm dialog', () => {
    expect(isPageClickOutside(at() as unknown as EventTarget)).toBe(false)
  })
  it('ignores a click inside the drawer', () => {
    expect(isPageClickOutside(at('#app', '.quick-edit') as unknown as EventTarget)).toBe(false)
  })
  it('ignores a click with no element target', () => {
    expect(isPageClickOutside(null)).toBe(false)
  })
})
