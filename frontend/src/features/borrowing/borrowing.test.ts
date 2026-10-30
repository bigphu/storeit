import { describe, expect, it } from 'vitest'
import {
  canSeeTicket,
  isBorrowingStatus,
  ON_LOAN_STATUS_ID,
  RESERVED_STATUS_ID,
  TICKET_STATE_LABEL,
  TICKET_STATE_TONE,
  overdueLabel,
  parseRequirement,
  pickableStatuses,
  requirementLabels,
  satisfies,
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
  it('colours states by meaning', () => {
    expect(TICKET_STATE_TONE).toEqual({ waiting: 'warn', approved: 'info', handed_over: 'success', rejected: 'danger', cancelled: 'secondary' })
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
  it('reads a condition on a removed attribute as written', () => {
    const removed = [{ ...attrs[0]!, removed: true }]
    expect(requirementLabels(['ram:gte:16'], removed)).toEqual(['ram:gte:16'])
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

describe('satisfies', () => {
  const v = (data_type: string, value: unknown) => ({ key: 'k', label: 'K', data_type, value }) as never
  it('compares numbers, dates and text like the list filter', () => {
    expect(satisfies({ key: 'k', op: 'gte', value: '16' }, v('number', 16))).toBe(true)
    expect(satisfies({ key: 'k', op: 'gte', value: '16' }, v('number', 8))).toBe(false)
    expect(satisfies({ key: 'k', op: 'lt', value: '2026-01-01' }, v('date', '2025-12-31'))).toBe(true)
    expect(satisfies({ key: 'k', op: 'eq', value: 'dell' }, v('text', 'Dell'))).toBe(true)
    expect(satisfies({ key: 'k', op: 'contains', value: 'HDMI' }, v('text', 'usb-c, hdmi'))).toBe(true)
  })
  it('checks booleans and select options', () => {
    expect(satisfies({ key: 'k', op: 'eq', value: 'true' }, v('boolean', true))).toBe(true)
    expect(satisfies({ key: 'k', op: 'in', value: 'o1,o2' }, v('select', 'o2'))).toBe(true)
    expect(satisfies({ key: 'k', op: 'eq', value: 'o1' }, v('select', 'o2'))).toBe(false)
  })
  it('never matches an empty value', () => {
    expect(satisfies({ key: 'k', op: 'eq', value: 'x' }, v('text', null))).toBe(false)
    expect(satisfies({ key: 'k', op: 'eq', value: 'x' }, undefined)).toBe(false)
  })
})

describe('canSeeTicket', () => {
  const b = { account: { id: 'ha' } }
  it('lets managers and the borrower open the ticket, nobody else', () => {
    expect(canSeeTicket(b, 'someone', true)).toBe(true)
    expect(canSeeTicket(b, 'ha', false)).toBe(true)
    expect(canSeeTicket(b, 'an', false)).toBe(false)
    expect(canSeeTicket(null, 'ha', true)).toBe(false)
  })
})
