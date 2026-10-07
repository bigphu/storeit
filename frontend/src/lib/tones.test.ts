import { describe, expect, it } from 'vitest'
import { accountStatusTone, statusKindTone } from './tones'

describe('tones', () => {
  it('maps status kinds to the spec tag roles', () => {
    expect(statusKindTone('available')).toBe('success')
    expect(statusKindTone('in_use')).toBe('info')
    expect(statusKindTone('unavailable')).toBe('warn')
    expect(statusKindTone('retired')).toBe('danger')
  })
  it('maps account statuses to the spec tag roles', () => {
    expect(accountStatusTone('active')).toBe('success')
    expect(accountStatusTone('invited')).toBe('info')
    expect(accountStatusTone('disabled')).toBe('danger')
  })
})
