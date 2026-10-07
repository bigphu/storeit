import { afterEach, describe, expect, it, vi } from 'vitest'
import { clickOrDouble } from './clickOrDouble'

const plain = { ctrlKey: false, metaKey: false, button: 0 }
afterEach(() => vi.useRealTimers())

describe('clickOrDouble', () => {
  it('opens after the delay on a single click', () => {
    vi.useFakeTimers()
    const open = vi.fn()
    const c = clickOrDouble(220)
    c.click(plain, open)
    vi.advanceTimersByTime(219)
    expect(open).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(open).toHaveBeenCalledOnce()
  })
  it('never opens when a double click follows', () => {
    vi.useFakeTimers()
    const open = vi.fn()
    const c = clickOrDouble(220)
    c.click(plain, open)
    c.click(plain, open)
    c.double()
    vi.advanceTimersByTime(1000)
    expect(open).not.toHaveBeenCalled()
  })
  it('opens at once on Ctrl/⌘ click and middle click', () => {
    const open = vi.fn()
    const c = clickOrDouble(220)
    c.click({ ...plain, ctrlKey: true }, open)
    c.click({ ...plain, metaKey: true }, open)
    c.click({ ...plain, button: 1 }, open)
    expect(open).toHaveBeenCalledTimes(3)
  })
})
