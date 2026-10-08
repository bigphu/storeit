import { keepPreviousData, type QueryClient, useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, type MaybeRefOrGetter, toValue } from 'vue'
import { inventoryApi } from '@/lib/api/client'
import { unwrap } from '@/lib/errors'
import { typeKeys } from '@/features/asset-types/api'
import type { toApiParams } from './listQuery'

export type ListParams = ReturnType<typeof toApiParams>

export const assetKeys = {
  all: ['assets'] as const,
  list: (p: ListParams) => ['assets', 'list', p] as const,
  one: (id: string) => ['assets', id] as const,
}

// assetListQuery: khoá và hàm tải dùng chung cho useAssetList, fetchAssetPage và tải trước
export function assetListQuery(params: ListParams) {
  return {
    queryKey: assetKeys.list(params),
    queryFn: () => unwrap(inventoryApi.GET('/assets', { params: { query: params } })),
  }
}

export function useAssetList(params: MaybeRefOrGetter<ListParams>) {
  return useQuery({
    queryKey: computed(() => assetListQuery(toValue(params)).queryKey),
    queryFn: () => assetListQuery(toValue(params)).queryFn(),
    placeholderData: keepPreviousData,
  })
}

// fetchAssetPage: tải một trang của danh sách (dùng cache nếu có), khi bước qua tài sản
// sang trang bên cạnh
export function fetchAssetPage(qc: QueryClient, params: ListParams) {
  return qc.fetchQuery(assetListQuery(params))
}

// assetQuery: khoá và hàm tải của một tài sản, dùng chung cho useAsset và tải trước khi rê chuột
export function assetQuery(id: string) {
  return {
    queryKey: assetKeys.one(id),
    queryFn: () => unwrap(inventoryApi.GET('/assets/{assetID}', { params: { path: { assetID: id } } })),
  }
}

export function useAsset(id: MaybeRefOrGetter<string | undefined>) {
  return useQuery({
    queryKey: computed(() => assetQuery(toValue(id) ?? '').queryKey),
    queryFn: () => assetQuery(toValue(id)!).queryFn(),
    enabled: computed(() => !!toValue(id)),
  })
}

export interface AssetBody {
  name: string
  description: string
  asset_type_id: string
  status_id?: string
  location_id?: string
  holder_member_id?: string
  purchase_date?: string
  attributes: Record<string, unknown>
}

function useAssetMutation<V, R>(fn: (v: V) => Promise<R>, toast = true) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: fn,
    meta: { toast },
    // số tài sản theo loại (sidebar) cũng đổi
    onSuccess: () => refreshAssetLists(qc),
  })
}

const path = (id: string) => ({ params: { path: { assetID: id } } })

export function useCreateAsset() {
  return useAssetMutation(
    (body: AssetBody & { tag: string }) => unwrap(inventoryApi.POST('/assets', { body })),
    false,
  )
}

// PUT thay toàn bộ: trường thiếu bị xoá, nên gửi đủ cả trường form không hiện
export function useReplaceAsset() {
  return useAssetMutation(
    ({ id, ...body }: AssetBody & { id: string; version: number }) =>
      unwrap(inventoryApi.PUT('/assets/{assetID}', { ...path(id), body })),
    false,
  )
}

export function useRetireAsset() {
  return useAssetMutation(
    ({ id, reason, version }: { id: string; reason: string; version: number }) =>
      unwrap(inventoryApi.POST('/assets/{assetID}/retire', { ...path(id), body: { reason, version } })),
    false,
  )
}

// restoreAssetRequest: chỉ gọi API, không nạp lại danh sách (Undo của retire hàng loạt gọi
// nhiều lần rồi nạp lại một lần, xem refreshAssetLists)
export function restoreAssetRequest({ id, version }: { id: string; version: number }) {
  return unwrap(inventoryApi.POST('/assets/{assetID}/restore', { ...path(id), body: { version } }))
}

export function useRestoreAsset() {
  return useAssetMutation(restoreAssetRequest, false)
}

// refreshAssetLists: nạp lại danh sách tài sản và số tài sản theo loại (sidebar)
export function refreshAssetLists(qc: QueryClient) {
  return Promise.all([qc.invalidateQueries({ queryKey: assetKeys.all }), qc.invalidateQueries({ queryKey: typeKeys.all })])
}

// fetchAsset: đọc một lần ngoài query (lý do retire cho Undo của restore)
export function fetchAsset(id: string) {
  return unwrap(inventoryApi.GET('/assets/{assetID}', path(id)))
}

// Hàng loạt: mỗi tài sản thành công hay thất bại riêng; kết quả liệt kê từng cái
export interface BulkItemRef {
  id: string
  version: number
}

export function useBulkRetire() {
  return useAssetMutation(
    ({ items, reason }: { items: BulkItemRef[]; reason: string }) =>
      unwrap(inventoryApi.POST('/assets/bulk-retire', { body: { items, reason: reason || undefined } })),
    false,
  )
}

export function useBulkStatus() {
  return useAssetMutation(
    ({ items, statusId }: { items: BulkItemRef[]; statusId: string }) =>
      unwrap(inventoryApi.POST('/assets/bulk-status', { body: { items, status_id: statusId } })),
    false,
  )
}
