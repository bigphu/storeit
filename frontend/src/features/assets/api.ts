import { keepPreviousData, type QueryClient, useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, type MaybeRefOrGetter, toValue } from 'vue'
import { inventoryApi } from '@/lib/api/client'
import { unwrap } from '@/lib/errors'
import type { toApiParams } from './listQuery'

export type ListParams = ReturnType<typeof toApiParams>

export const assetKeys = {
  all: ['assets'] as const,
  list: (p: ListParams) => ['assets', 'list', p] as const,
  one: (id: string) => ['assets', id] as const,
}

export function useAssetList(params: MaybeRefOrGetter<ListParams>) {
  return useQuery({
    queryKey: computed(() => assetKeys.list(toValue(params))),
    queryFn: () => unwrap(inventoryApi.GET('/assets', { params: { query: toValue(params) } })),
    placeholderData: keepPreviousData,
  })
}

// fetchAssetPage: tải một trang của danh sách (dùng cache nếu có), khi bước qua tài sản
// sang trang bên cạnh
export function fetchAssetPage(qc: QueryClient, params: ListParams) {
  return qc.fetchQuery({
    queryKey: assetKeys.list(params),
    queryFn: () => unwrap(inventoryApi.GET('/assets', { params: { query: params } })),
  })
}

export function useAsset(id: MaybeRefOrGetter<string | undefined>) {
  return useQuery({
    queryKey: computed(() => assetKeys.one(toValue(id) ?? '')),
    queryFn: () => unwrap(inventoryApi.GET('/assets/{assetID}', { params: { path: { assetID: toValue(id)! } } })),
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
    onSuccess: () => qc.invalidateQueries({ queryKey: assetKeys.all }),
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
  return useAssetMutation(({ id, reason, version }: { id: string; reason: string; version: number }) =>
    unwrap(inventoryApi.POST('/assets/{assetID}/retire', { ...path(id), body: { reason, version } })),
  )
}

export function useRestoreAsset() {
  return useAssetMutation(({ id, version }: { id: string; version: number }) =>
    unwrap(inventoryApi.POST('/assets/{assetID}/restore', { ...path(id), body: { version } })),
  )
}
