import { describe, expect, it, vi } from 'vitest'
import { countdown, holds } from './countdown'

describe('countdown', () => {
  it('counts down, pauses and resumes', () => {
    const c = countdown(1000, 0)
    expect(c.left(250)).toBeCloseTo(0.75)
    c.pause(250)
    expect(c.left(900)).toBeCloseTo(0.75)
    c.pause(950) // tạm dừng lần nữa không đổi gì
    c.resume(900)
    expect(c.left(1150)).toBeCloseTo(0.5)
    c.resume(1150) // đang chạy: không đổi gì
    expect(c.left(5000)).toBe(0)
  })
})

// Dừng khi có lý do (chuột ở trên, focus ở trong), chạy lại khi không còn lý do nào
describe('holds', () => {
  it('resumes only when neither hover nor focus holds it', () => {
    const pause = vi.fn()
    const resume = vi.fn()
    const h = holds(pause, resume)
    h.hold('hover')
    h.hold('focus')
    h.release('hover')
    expect(resume).not.toHaveBeenCalled()
    h.release('focus')
    expect(resume).toHaveBeenCalledOnce()
    expect(pause).toHaveBeenCalledOnce()
  })
})
