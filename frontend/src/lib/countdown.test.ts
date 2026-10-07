import { describe, expect, it } from 'vitest'
import { countdown } from './countdown'

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
