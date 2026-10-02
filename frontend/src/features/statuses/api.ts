import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, type MaybeRefOrGetter, toValue } from 'vue'
import { inventoryApi } from '@/lib/api/client'
import type { StatusKind } from '@/lib/api/types'
import { unwrap } from '@/lib/errors'

export const statusKeys = {
  all: ['statuses'] as const,
  list: (archived: boolean) => ['statuses', archived] as const,
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

export function useStatuses(includeArchived: MaybeRefOrGetter<boolean> = false) {
  return useQuery({
    queryKey: computed(() => statusKeys.list(toValue(includeArchived))),
    queryFn: async () =>
      (
        await unwrap(
          inventoryApi.GET('/asset-statuses', { params: { query: { include_archived: toValue(includeArchived) } } }),
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
