import { describe, expect, it } from 'vitest'
import { earliestReplacementDay, replacementFormError, replacementGroups, todayLocal } from './replacement'

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
  it('refuses a date before either asset was purchased', () => {
    const purchased = [
      { tag: 'LAP-0001', on: '2026-10-10' },
      { tag: 'LAP-0042', on: '2026-10-15' },
      { tag: 'LAP-0099' },
    ]
    expect(replacementFormError({ ...ok, replacedOn: '2026-10-14', purchased }, today)).toBe(
      "The date can't be before LAP-0042 was purchased (2026-10-15).",
    )
    expect(replacementFormError({ ...ok, replacedOn: '2026-10-09', purchased }, today)).toBe(
      "The date can't be before LAP-0001 was purchased (2026-10-10).",
    )
    expect(replacementFormError({ ...ok, replacedOn: '2026-10-15', purchased }, today)).toBeNull()
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

describe('replacementGroups', () => {
  const item = (id: string, typeId: string, typeName: string) => ({ id, tag: id.toUpperCase(), asset_type_id: typeId, asset_type_name: typeName })
  it('drops the asset itself and lists the same type first', () => {
    const items = [item('a', 'mon', 'Monitor'), item('self', 'lap', 'Laptop'), item('b', 'lap', 'Laptop'), item('c', 'lap', 'Laptop')]
    expect(replacementGroups(items, { id: 'self', typeId: 'lap' })).toEqual([
      { label: 'Laptop', items: [items[2], items[3]] },
      { label: 'Other types', items: [items[0]] },
    ])
  })
  it('leaves out empty groups', () => {
    expect(replacementGroups([item('a', 'mon', 'Monitor')], { id: 'self', typeId: 'lap' })).toEqual([
      { label: 'Other types', items: [{ id: 'a', tag: 'A', asset_type_id: 'mon', asset_type_name: 'Monitor' }] },
    ])
    expect(replacementGroups([], { id: 'self', typeId: 'lap' })).toEqual([])
  })
})

describe('earliestReplacementDay', () => {
  it('is the latest purchase date, if any', () => {
    expect(earliestReplacementDay([{ tag: 'A', on: '2026-10-10' }, { tag: 'B', on: '2026-10-15' }, { tag: 'C' }])).toBe('2026-10-15')
    expect(earliestReplacementDay([{ tag: 'A' }])).toBeUndefined()
  })
})
