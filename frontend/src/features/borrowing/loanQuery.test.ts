import { describe, expect, it } from 'vitest'
import { LOAN_SHOWS, type LoanListState, loanEmptyText, loanParams, loanRange, parseLoanQuery, serializeLoanQuery } from './loanQuery'

const base: LoanListState = { show: 'active', period: 'any', from: '', to: '', q: '', type: '', page: 1 }
const today = '2026-11-10'

describe('URL <-> state', () => {
  it('starts on Out now with no filters', () => {
    expect(parseLoanQuery({})).toEqual(base)
    expect(serializeLoanQuery(base)).toEqual({
      show: undefined, period: undefined, from: undefined, to: undefined, q: undefined, type: undefined, sort: undefined, page: undefined,
    })
  })

  it('reads and writes every filter, so a reloaded or shared link shows the same list', () => {
    const query = { show: 'returned_late', period: 'custom', from: '2026-10-01', to: '2026-10-31', q: 'dell', type: 'T1', sort: '-return_by', page: '3' }
    const state = parseLoanQuery(query)
    expect(state).toEqual({ show: 'returned_late', period: 'custom', from: '2026-10-01', to: '2026-10-31', q: 'dell', type: 'T1', sort: '-return_by', page: 3 })
    expect(serializeLoanQuery(state)).toEqual(query)
  })

  it('knows all five statuses', () => {
    for (const show of LOAN_SHOWS) expect(parseLoanQuery({ show }).show).toBe(show)
    expect(LOAN_SHOWS).toEqual(['active', 'overdue', 'returned', 'returned_late', 'all'])
  })

  it('falls back to the defaults on values it does not know, instead of sending them to the API', () => {
    const s = parseLoanQuery({ show: 'late', period: 'yesterday', sort: 'borrower', page: '0' })
    expect(s).toMatchObject({ show: 'active', period: 'any', page: 1 })
    expect(s.sort).toBeUndefined()
    expect(parseLoanQuery({ page: 'x' }).page).toBe(1)
    expect(parseLoanQuery({ page: '-2' }).page).toBe(1)
    expect(parseLoanQuery({ show: ['overdue', 'all'] }).show).toBe('overdue')
  })

  it('keeps from and to only for a custom range', () => {
    const q = serializeLoanQuery({ ...base, period: 'last_30', from: '2026-10-01', to: '2026-10-31' })
    expect(q.from).toBeUndefined()
    expect(q.to).toBeUndefined()
    expect(q.period).toBe('last_30')
  })

  it('leaves page 1 and the default status off the URL', () => {
    expect(serializeLoanQuery({ ...base, show: 'all', page: 1 })).toMatchObject({ show: 'all', page: undefined })
    expect(serializeLoanQuery({ ...base, page: 2 }).page).toBe('2')
  })
})

describe('loanRange', () => {
  it('is empty for any time', () => {
    expect(loanRange(base, today)).toEqual({})
  })
  it('counts the preset periods from the business day', () => {
    expect(loanRange({ ...base, period: 'this_month' }, today)).toEqual({ from: '2026-11-01', to: today })
    expect(loanRange({ ...base, period: 'last_month' }, today)).toEqual({ from: '2026-10-01', to: '2026-10-31' })
    expect(loanRange({ ...base, period: 'last_30' }, today)).toEqual({ from: '2026-10-12', to: today })
  })
  it('takes a custom range as typed, and an open end as no limit', () => {
    expect(loanRange({ ...base, period: 'custom', from: '2026-10-01', to: '2026-10-05' }, today)).toEqual({ from: '2026-10-01', to: '2026-10-05' })
    expect(loanRange({ ...base, period: 'custom', from: '2026-10-01' }, today)).toEqual({ from: '2026-10-01', to: undefined })
  })
})

describe('loanParams', () => {
  it('sends the status, type, search and paging the API understands', () => {
    const p = loanParams({ ...base, show: 'overdue', q: 'dell', type: 'T1', sort: 'return_by', page: 2 }, { today, pageSize: 25 })
    expect(p).toEqual({ mine: undefined, state: 'overdue', q: 'dell', type_id: 'T1', sort: 'return_by', page: 2, page_size: 25 })
  })
  it('sends every status as the API names it', () => {
    for (const show of LOAN_SHOWS) expect(loanParams({ ...base, show }, { today, pageSize: 50 }).state).toBe(show)
  })
  it('asks for my loans only on My loans', () => {
    expect(loanParams(base, { mine: true, today, pageSize: 50 }).mine).toBe(true)
    expect(loanParams(base, { today, pageSize: 50 }).mine).toBeUndefined()
  })
  it('leaves out an empty search and an empty type', () => {
    const p = loanParams(base, { today, pageSize: 50 })
    expect(p.q).toBeUndefined()
    expect(p.type_id).toBeUndefined()
  })
  it('adds the period as from and to', () => {
    expect(loanParams({ ...base, period: 'this_month' }, { today, pageSize: 50 })).toMatchObject({ from: '2026-11-01', to: today })
  })
})

describe('loanEmptyText', () => {
  it('says the filters are the reason when any is set', () => {
    for (const s of [{ q: 'zzz' }, { type: 'T1' }, { period: 'last_30' as const }]) {
      expect(loanEmptyText({ ...base, ...s })).toBe('No loans match these filters.')
      expect(loanEmptyText({ ...base, ...s }, true)).toBe('No loans match these filters.')
    }
  })
  it('speaks to the person on My loans', () => {
    expect(loanEmptyText(base, true)).toBe('You have nothing on loan.')
    expect(loanEmptyText({ ...base, show: 'returned' }, true)).toBe("You haven't returned anything yet.")
  })
  it('speaks about the store on Loans', () => {
    expect(loanEmptyText(base)).toBe('Nothing is on loan right now.')
    expect(loanEmptyText({ ...base, show: 'all' })).toBe('No loans yet.')
  })
})
