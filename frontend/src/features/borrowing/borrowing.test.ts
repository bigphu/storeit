import { describe, expect, it } from 'vitest'
import {
  isBorrowingStatus,
  ON_LOAN_STATUS_ID,
  RESERVED_STATUS_ID,
  TICKET_STATE_LABEL,
  overdueLabel,
  parseRequirement,
  pickableStatuses,
  requirementLabels,
  shortDay,
  ticketFormError,
} from './borrowing'

describe('pickableStatuses', () => {
  it('leaves out Reserved and On loan, which only borrowing sets', () => {
    const all = [{ id: 'avail' }, { id: RESERVED_STATUS_ID }, { id: 'repair' }, { id: ON_LOAN_STATUS_ID }]
    expect(pickableStatuses(all)).toEqual([{ id: 'avail' }, { id: 'repair' }])
  })
  it('knows the two borrowing statuses', () => {
    expect(isBorrowingStatus(RESERVED_STATUS_ID) && isBorrowingStatus(ON_LOAN_STATUS_ID)).toBe(true)
    expect(isBorrowingStatus('avail') || isBorrowingStatus(undefined)).toBe(false)
  })
})

describe('labels', () => {
  it('names every ticket state', () => {
    expect(TICKET_STATE_LABEL).toEqual({
      waiting: 'Waiting',
      approved: 'Ready for pickup',
      rejected: 'Rejected',
      cancelled: 'Cancelled',
      handed_over: 'Handed over',
    })
  })
  it('says how overdue a loan is', () => {
    expect(overdueLabel(0)).toBe('')
    expect(overdueLabel(1)).toBe('Overdue 1 day')
    expect(overdueLabel(5)).toBe('Overdue 5 days')
  })
  it('shows a date as day and short month', () => {
    expect(shortDay('2026-11-07')).toBe('7 Nov')
    expect(shortDay(undefined)).toBe('')
  })
})

describe('requirements', () => {
  const attrs = [
    { key: 'ram', label: 'RAM', data_type: 'number' as const, unit: 'GB', options: [] },
    { key: 'os', label: 'OS', data_type: 'select' as const, options: [{ id: 'o1', label: 'Windows' }, { id: 'o2', label: 'macOS' }] },
  ]
  it('splits a condition at the first two colons', () => {
    expect(parseRequirement('note:contains:a:b')).toEqual({ key: 'note', op: 'contains', value: 'a:b' })
  })
  it('reads conditions like the filter chips; unknown ones stay as written', () => {
    expect(requirementLabels(['ram:gte:16', 'os:in:o1,o2', 'gone:eq:x'], attrs)).toEqual(['RAM ≥ 16 GB', 'OS any of Windows, macOS', 'gone:eq:x'])
  })
})

describe('ticketFormError', () => {
  const ok = { typeId: 't1', neededFrom: '2026-10-30', note: '' }
  const today = '2026-10-30'
  it('names the first problem', () => {
    expect(ticketFormError({ ...ok, typeId: undefined }, today)).toBe('Pick what you need.')
    expect(ticketFormError({ ...ok, neededFrom: undefined }, today)).toBe('Pick the date you need it from.')
    expect(ticketFormError({ ...ok, neededFrom: '2026-10-29' }, today)).toBe('That date has passed.')
    expect(ticketFormError({ ...ok, neededFrom: '2026-11-02', returnBy: '2026-11-01' }, today)).toBe(
      "Return by can't be before the date you need it from.",
    )
    expect(ticketFormError({ ...ok, note: 'x'.repeat(501) }, today)).toBe('Keep the note under 500 characters.')
  })
  it('accepts a complete ticket', () => {
    expect(ticketFormError(ok, today)).toBeNull()
    expect(ticketFormError({ ...ok, returnBy: '2026-10-30', note: 'x'.repeat(500) }, today)).toBeNull()
  })
})
