import { describe, expect, it } from 'vitest'
import { replacementFormError, todayLocal } from './replacement'

describe('replacementFormError', () => {
  const ok = { replacementId: 'n1', replacedOn: '2026-10-29', reason: 'Broken screen' }
  const today = '2026-10-29'
  it('names the first missing or wrong field', () => {
    expect(replacementFormError({ ...ok, replacementId: undefined }, today)).toBe('Pick the replacement asset.')
    expect(replacementFormError({ ...ok, replacedOn: undefined }, today)).toBe('Pick the date.')
    expect(replacementFormError({ ...ok, replacedOn: '2026-10-30' }, today)).toBe("The date can't be in the future.")
    expect(replacementFormError({ ...ok, reason: '   ' }, today)).toBe('Give a reason.')
    expect(replacementFormError({ ...ok, reason: 'x'.repeat(501) }, today)).toBe('Keep the reason under 500 characters.')
  })
  it('accepts a complete form', () => {
    expect(replacementFormError(ok, today)).toBeNull()
    expect(replacementFormError({ ...ok, reason: 'x'.repeat(500) }, today)).toBeNull()
  })
})

describe('todayLocal', () => {
  it('is the local calendar day', () => {
    expect(todayLocal(new Date(2026, 9, 29, 23, 30))).toBe('2026-10-29')
    expect(todayLocal(new Date(2026, 0, 5, 0, 5))).toBe('2026-01-05')
  })
})
