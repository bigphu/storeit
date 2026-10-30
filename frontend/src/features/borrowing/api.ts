// Mượn tài sản (US-10): phiếu mượn, khoản đang mượn, cho mượn trực tiếp. Mutation không tự
// toast (meta toast false): trang và hộp thoại tự báo, lỗi field hiện trong form
import { keepPreviousData, type QueryClient, useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, type MaybeRefOrGetter, toValue } from 'vue'
import { inventoryApi } from '@/lib/api/client'
import type { operations } from '@/lib/api/inventory'
import type { Attribute } from '@/lib/api/types'
import { unwrap } from '@/lib/errors'
import { refreshAssetLists } from '@/features/assets/api'
import { typeKeys } from '@/features/asset-types/api'

export type TicketParams = NonNullable<operations['listBorrowTickets']['parameters']['query']>
export type LoanParams = NonNullable<operations['listLoans']['parameters']['query']>

export const borrowKeys = {
  all: ['borrowing'] as const,
  tickets: (p: TicketParams) => ['borrowing', 'tickets', p] as const,
  ticket: (id: string) => ['borrowing', 'ticket', id] as const,
  counts: ['borrowing', 'counts'] as const,
  loans: (p: LoanParams) => ['borrowing', 'loans', p] as const,
}

export function useTickets(params: MaybeRefOrGetter<TicketParams>) {
  return useQuery({
    queryKey: computed(() => borrowKeys.tickets(toValue(params))),
    queryFn: () => unwrap(inventoryApi.GET('/borrow-tickets', { params: { query: toValue(params) } })),
    placeholderData: keepPreviousData,
  })
}

export function useTicket(id: MaybeRefOrGetter<string | undefined>) {
  return useQuery({
    queryKey: computed(() => borrowKeys.ticket(toValue(id) ?? '')),
    queryFn: () => unwrap(inventoryApi.GET('/borrow-tickets/{ticketID}', { params: { path: { ticketID: toValue(id)! } } })),
    enabled: computed(() => !!toValue(id)),
  })
}

// useTicketCounts: số phiếu chờ duyệt cho sidebar (chỉ người quản lý), tự tải lại mỗi phút
export function useTicketCounts(enabled: MaybeRefOrGetter<boolean>) {
  return useQuery({
    queryKey: borrowKeys.counts,
    queryFn: () => unwrap(inventoryApi.GET('/borrow-tickets/summary')),
    enabled: computed(() => toValue(enabled)),
    refetchInterval: 60_000,
  })
}

export function useLoans(params: MaybeRefOrGetter<LoanParams>) {
  return useQuery({
    queryKey: computed(() => borrowKeys.loans(toValue(params))),
    queryFn: () => unwrap(inventoryApi.GET('/loans', { params: { query: toValue(params) } })),
    placeholderData: keepPreviousData,
  })
}

// searchBorrowers: account đang hoạt động để chọn người mượn (hộp Lend)
export async function searchBorrowers(q: string) {
  return (await unwrap(inventoryApi.GET('/borrowers', { params: { query: { q: q || undefined } } }))).items
}

// refreshBorrowing: phiếu, khoản mượn và tài sản (status đổi theo) đều tải lại
export function refreshBorrowing(qc: QueryClient) {
  return Promise.all([qc.invalidateQueries({ queryKey: borrowKeys.all }), refreshAssetLists(qc)])
}

function useBorrowMutation<V, R>(fn: (v: V) => Promise<R>) {
  const qc = useQueryClient()
  return useMutation({ mutationFn: fn, meta: { toast: false }, onSuccess: () => refreshBorrowing(qc) })
}

const ticketPath = (id: string) => ({ params: { path: { ticketID: id } } })

export interface NewTicket {
  asset_type_id: string
  requirements: string[]
  suggested_asset_id?: string
  needed_from: string
  return_by?: string
  note?: string
}

export function useCreateTicket() {
  return useBorrowMutation((body: NewTicket) => unwrap(inventoryApi.POST('/borrow-tickets', { body })))
}

export function useApproveTicket() {
  return useBorrowMutation(({ id, assetId, version }: { id: string; assetId: string; version: number }) =>
    unwrap(inventoryApi.POST('/borrow-tickets/{ticketID}/approve', { ...ticketPath(id), body: { asset_id: assetId, version } })),
  )
}

export function useRejectTicket() {
  return useBorrowMutation(({ id, reason, version }: { id: string; reason: string; version: number }) =>
    unwrap(inventoryApi.POST('/borrow-tickets/{ticketID}/reject', { ...ticketPath(id), body: { reason, version } })),
  )
}

export function useCancelTicket() {
  return useBorrowMutation(({ id, reason, version }: { id: string; reason?: string; version: number }) =>
    unwrap(inventoryApi.POST('/borrow-tickets/{ticketID}/cancel', { ...ticketPath(id), body: { reason: reason || undefined, version } })),
  )
}

export function useHandOver() {
  return useBorrowMutation(({ id, version }: { id: string; version: number }) =>
    unwrap(inventoryApi.POST('/borrow-tickets/{ticketID}/hand-over', { ...ticketPath(id), body: { version } })),
  )
}

export function useLendAsset() {
  return useBorrowMutation(({ id, borrowerId, returnBy, version }: { id: string; borrowerId: string; returnBy?: string; version: number }) =>
    unwrap(
      inventoryApi.POST('/assets/{assetID}/lend', {
        params: { path: { assetID: id } },
        body: { borrower_id: borrowerId, return_by: returnBy || undefined, version },
      }),
    ),
  )
}

// useTypeAttributes: thuộc tính của các loại (đọc yêu cầu của phiếu thành chữ), mỗi loại một
// lần tải, dùng chung cache với trang loại
export function useTypeAttributes(ids: MaybeRefOrGetter<string[]>) {
  const results = useQueries({
    queries: computed(() =>
      [...new Set(toValue(ids))].map((id) => ({
        queryKey: typeKeys.one(id),
        queryFn: () => unwrap(inventoryApi.GET('/asset-types/{typeID}', { params: { path: { typeID: id } } })),
      })),
    ),
  })
  return computed(() => {
    const out: Record<string, Attribute[]> = {}
    for (const r of results.value) if (r.data) out[r.data.id] = r.data.attributes
    return out
  })
}
