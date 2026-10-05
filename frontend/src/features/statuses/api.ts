import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, type MaybeRefOrGetter, toValue } from 'vue'
import { inventoryApi } from '@/lib/api/client'
import type { Status, StatusKind } from '@/lib/api/types'
import { unwrap } from '@/lib/errors'

export const statusKeys = {
  all: ['statuses'] as const,
  list: (archived: boolean, counts = false) => ['statuses', archived, counts] as const,
}

export const statusKinds: StatusKind[] = ['available', 'in_use', 'unavailable', 'retired']

// Màu tag theo kind
export function kindSeverity(k: StatusKind): 'success' | 'info' | 'warn' | 'secondary' {
  return { available: 'success', in_use: 'info', unavailable: 'warn', retired: 'secondary' }[k] as
    | 'success'
    | 'info'
    | 'warn'
    | 'secondary'
}

// withCounts: kèm asset_count (số tài sản của mỗi status, kể cả đã retire) cho trang status
export function useStatuses(includeArchived: MaybeRefOrGetter<boolean> = false, withCounts = false) {
  return useQuery({
    queryKey: computed(() => statusKeys.list(toValue(includeArchived), withCounts)),
    queryFn: async () =>
      (
        await unwrap(
          inventoryApi.GET('/asset-statuses', {
            params: { query: { include_archived: toValue(includeArchived), with_counts: withCounts || undefined } },
          }),
        )
      ).items,
  })
}

function useStatusMutation<V, R>(fn: (v: V) => Promise<R>, toast = true) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: fn,
    meta: { toast },
    onSuccess: () => qc.invalidateQueries({ queryKey: statusKeys.all }),
  })
}

export function useCreateStatus() {
  return useStatusMutation(
    (body: { name: string; kind: StatusKind; position: number }) =>
      unwrap(inventoryApi.POST('/asset-statuses', { body })),
    false,
  )
}

export function useUpdateStatus() {
  return useStatusMutation(
    ({ id, ...body }: { id: string; name?: string; position?: number; make_default?: boolean }) =>
      unwrap(inventoryApi.PATCH('/asset-statuses/{statusID}', { params: { path: { statusID: id } }, body })),
    false,
  )
}

export function useArchiveStatus() {
  return useStatusMutation((id: string) =>
    unwrap(inventoryApi.POST('/asset-statuses/{statusID}/archive', { params: { path: { statusID: id } } })),
  )
}

export function useRestoreStatus() {
  return useStatusMutation((id: string) =>
    unwrap(inventoryApi.POST('/asset-statuses/{statusID}/restore', { params: { path: { statusID: id } } })),
  )
}

// useReorderStatuses: ids là mọi status đang dùng theo thứ tự mới. Đổi thứ tự trong cache
// ngay (kéo thả không giật), lỗi thì nạp lại từ server.
export function useReorderStatuses() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (ids: string[]) => unwrap(inventoryApi.PUT('/asset-statuses/order', { body: { ids } })),
    meta: { toast: true },
    onMutate: async (ids: string[]) => {
      await qc.cancelQueries({ queryKey: statusKeys.all })
      const rank = new Map(ids.map((id, i) => [id, i + 1]))
      qc.setQueriesData<Status[]>({ queryKey: statusKeys.all }, (list) =>
        list
          ?.map((s) => (rank.has(s.id) ? { ...s, position: rank.get(s.id)! } : s))
          .sort((a, b) => a.position - b.position),
      )
    },
    onSettled: () => qc.invalidateQueries({ queryKey: statusKeys.all }),
  })
}
