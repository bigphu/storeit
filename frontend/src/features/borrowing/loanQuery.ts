// State của trang Loans / My loans <-> URL <-> tham số GET /loans (US-20).
// URL dùng: show (bỏ khi là Out now), period, from, to (chỉ khi Custom), q, type, sort, page.
import type { LocationQuery, LocationQueryRaw } from 'vue-router'
import { queryInt, queryString } from '@/lib/queryParams'
import type { LoanParams } from './api'
import { type Period, periodRange } from './borrowing'

export type LoanSort = NonNullable<LoanParams['sort']>
export type LoanShow = NonNullable<LoanParams['state']>
export const LOAN_SORTS: LoanSort[] = ['started_on', '-started_on', 'return_by', '-return_by', 'returned_on', '-returned_on']
export const LOAN_SHOWS: LoanShow[] = ['active', 'overdue', 'returned', 'returned_late', 'all']
export const LOAN_PERIODS: Period[] = ['any', 'this_month', 'last_month', 'last_30', 'custom']

export interface LoanListState {
  show: LoanShow
  period: Period
  from: string
  to: string
  q: string
  // id loại tài sản; rỗng là mọi loại
  type: string
  sort?: LoanSort
  page: number
}

// Giá trị lạ trên URL (gõ tay, link cũ) rơi về mặc định thay vì làm API báo lỗi
export function parseLoanQuery(q: LocationQuery): LoanListState {
  const sort = queryString(q.sort) as LoanSort | undefined
  const show = queryString(q.show) as LoanShow | undefined
  const period = queryString(q.period) as Period | undefined
  return {
    show: show && LOAN_SHOWS.includes(show) ? show : 'active',
    period: period && LOAN_PERIODS.includes(period) ? period : 'any',
    from: queryString(q.from) ?? '',
    to: queryString(q.to) ?? '',
    q: queryString(q.q) ?? '',
    type: queryString(q.type) ?? '',
    sort: sort && LOAN_SORTS.includes(sort) ? sort : undefined,
    page: queryInt(q.page, 1),
  }
}

// Mặc định không lên URL, để link ngắn và "không lọc" chỉ có một dạng
export function serializeLoanQuery(s: LoanListState): LocationQueryRaw {
  return {
    show: s.show === 'active' ? undefined : s.show,
    period: s.period === 'any' ? undefined : s.period,
    from: s.period === 'custom' && s.from ? s.from : undefined,
    to: s.period === 'custom' && s.to ? s.to : undefined,
    q: s.q || undefined,
    type: s.type || undefined,
    sort: s.sort,
    page: s.page > 1 ? String(s.page) : undefined,
  }
}

// loanRange: kỳ đang lọc; lựa chọn sẵn tính từ hôm nay của ứng dụng, Custom lấy from/to trên URL
export function loanRange(s: LoanListState, today: string): { from?: string; to?: string } {
  if (s.period === 'any') return {}
  if (s.period === 'custom') return { from: s.from || undefined, to: s.to || undefined }
  return periodRange(s.period, today)
}

export function loanParams(s: LoanListState, o: { mine?: boolean; today: string; pageSize: number }): LoanParams {
  return {
    mine: o.mine || undefined,
    state: s.show,
    ...loanRange(s, o.today),
    q: s.q || undefined,
    type_id: s.type || undefined,
    sort: s.sort,
    page: s.page,
    page_size: o.pageSize,
  }
}

export function loanEmptyText(s: LoanListState, mine?: boolean): string {
  if (s.q || s.type || s.period !== 'any') return 'No loans match these filters.'
  if (s.show === 'active') return mine ? 'You have nothing on loan.' : 'Nothing is on loan right now.'
  return mine ? "You haven't returned anything yet." : 'No loans yet.'
}
