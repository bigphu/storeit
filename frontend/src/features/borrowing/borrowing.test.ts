import { describe, expect, it } from 'vitest'
import type { Status } from '@/lib/api/types'
import {
  canSeeTicket,
  isBorrowingStatus,
  loanStateTag,
  periodRange,
  ON_LOAN_STATUS_ID,
  RESERVED_STATUS_ID,
  TICKET_STATE_LABEL,
  TICKET_STATE_TONE,
  overdueLabel,
  parseRequirement,
  pickableStatuses,
  requirementLabels,
  requestCode,
  RETURN_FINE,
  returnConditionOptions,
  returnFormError,
  ticketLoanLabel,
  satisfies,
  shortDay,
  ticketFormError,
} from './borrowing'

describe('requestCode', () => {
  it('shows a request number as an auto-generated code like asset tags', () => {
    expect(requestCode(4)).toBe('REQ-0004')
    expect(requestCode(12345)).toBe('REQ-12345')
  })
})

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

describe('returnConditionOptions', () => {
  const st = (id: string, kind: string, archived = false) =>
    ({ id, name: id, kind, archived_at: archived ? '2026-10-01T00:00:00Z' : null }) as unknown as Status
  // Select của PrimeVue coi null là "chưa chọn": Fine phải có giá trị thật để hiện sẵn
  it('gives Fine a real value so the select shows it as chosen', () => {
    expect(RETURN_FINE).toBeTruthy()
  })
  it('puts Fine first, then statuses an asset can come back damaged to', () => {
    const all = [
      st('Available', 'available'),
      st('Spare', 'available'),
      st('In repair', 'unavailable'),
      st(RESERVED_STATUS_ID, 'in_use'),
      st(ON_LOAN_STATUS_ID, 'in_use'),
      st('In use', 'in_use'),
      st('Lost', 'unavailable', true),
      st('Retired', 'retired'),
    ]
    expect(returnConditionOptions(all)).toEqual([
      { label: 'Fine — back to Available', value: RETURN_FINE },
      { label: 'In repair', value: 'In repair' },
      { label: 'In use', value: 'In use' },
    ])
  })
})

describe('returnFormError', () => {
  it('needs a date from the loan start to today', () => {
    expect(returnFormError('', '2026-10-20', '2026-10-30')).toBe('Pick the return date.')
    expect(returnFormError('2026-10-19', '2026-10-20', '2026-10-30')).toBe('The loan started on 20 Oct; pick that day or later.')
    expect(returnFormError('2026-10-31', '2026-10-20', '2026-10-30')).toBe("That date hasn't come yet.")
    expect(returnFormError('2026-10-20', '2026-10-20', '2026-10-30')).toBeNull()
  })
})

describe('ticketLoanLabel', () => {
  it('says whether the loan is still out', () => {
    expect(ticketLoanLabel({ started_on: '2026-11-01' })).toBe('On loan since 1 Nov')
    expect(ticketLoanLabel({ started_on: '2026-11-01', returned_on: '2026-11-03' })).toBe('Returned on 3 Nov')
  })
})

describe('periodRange', () => {
  it('turns a preset into dates from the business day', () => {
    expect(periodRange('this_month', '2026-10-31')).toEqual({ from: '2026-10-01', to: '2026-10-31' })
    expect(periodRange('last_month', '2026-10-31')).toEqual({ from: '2026-09-01', to: '2026-09-30' })
    expect(periodRange('last_30', '2026-10-31')).toEqual({ from: '2026-10-02', to: '2026-10-31' })
    expect(periodRange('last_month', '2026-01-15')).toEqual({ from: '2025-12-01', to: '2025-12-31' })
  })
})

describe('loanStateTag', () => {
  it('tells loans out from returned ones', () => {
    expect(loanStateTag({ overdue_days: 0 })).toEqual({ label: 'On loan', tone: 'info' })
    expect(loanStateTag({ overdue_days: 3, return_by: '2026-10-27' })).toEqual({ label: 'Overdue 3 days', tone: 'danger' })
    expect(loanStateTag({ overdue_days: 0, return_by: '2026-11-07', returned_on: '2026-11-05' })).toEqual({ label: 'Returned', tone: 'success' })
    expect(loanStateTag({ overdue_days: 0, return_by: '2026-11-07', returned_on: '2026-11-09' })).toEqual({ label: 'Returned late', tone: 'warn' })
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
