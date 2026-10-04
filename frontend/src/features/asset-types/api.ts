import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, type MaybeRefOrGetter, toValue } from 'vue'
import { inventoryApi } from '@/lib/api/client'
import type { DataType } from '@/lib/api/types'
import { unwrap } from '@/lib/errors'

export const typeKeys = {
  all: ['asset-types'] as const,
  list: (archived: boolean) => ['asset-types', 'list', archived] as const,
  one: (id: string) => ['asset-types', id] as const,
}

// withCounts: kèm asset_count (tài sản chưa retire) cho sidebar và bộ chọn loại
export function useAssetTypes(includeArchived: MaybeRefOrGetter<boolean> = false, withCounts = false) {
  return useQuery({
    queryKey: computed(() => [...typeKeys.list(toValue(includeArchived)), withCounts] as const),
    queryFn: async () =>
      (
        await unwrap(
          inventoryApi.GET('/asset-types', {
            params: { query: { include_archived: toValue(includeArchived), with_counts: withCounts || undefined } },
          }),
        )
      ).items,
  })
}

// useAssetType: kèm mọi thuộc tính (cả đã gỡ) và option; id rỗng thì không tải
export function useAssetType(id: MaybeRefOrGetter<string | undefined>) {
  return useQuery({
    queryKey: computed(() => typeKeys.one(toValue(id) ?? '')),
    queryFn: () =>
      unwrap(inventoryApi.GET('/asset-types/{typeID}', { params: { path: { typeID: toValue(id)! } } })),
    enabled: computed(() => !!toValue(id)),
  })
}

// Mọi thay đổi loại làm mới danh sách loại và loại đó; toast: false khi form tự hiện lỗi
function useTypeMutation<V, R>(fn: (v: V) => Promise<R>, toast = true) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: fn,
    meta: { toast },
    onSuccess: () => qc.invalidateQueries({ queryKey: typeKeys.all }),
  })
}

const typePath = (typeID: string) => ({ params: { path: { typeID } } })
const attrPath = (typeID: string, attributeID: string) => ({ params: { path: { typeID, attributeID } } })

export function useCreateAssetType() {
  return useTypeMutation(
    (body: { code: string; name: string; description?: string }) =>
      unwrap(inventoryApi.POST('/asset-types', { body })),
    false,
  )
}

export function useUpdateAssetType() {
  return useTypeMutation(
    ({ id, ...body }: { id: string; name: string; description: string; version: number }) =>
      unwrap(inventoryApi.PATCH('/asset-types/{typeID}', { ...typePath(id), body })),
    false,
  )
}

export function useArchiveAssetType() {
  return useTypeMutation((id: string) => unwrap(inventoryApi.POST('/asset-types/{typeID}/archive', typePath(id))))
}

export function useRestoreAssetType() {
  return useTypeMutation((id: string) => unwrap(inventoryApi.POST('/asset-types/{typeID}/restore', typePath(id))))
}

export interface AttributeInput {
  key: string
  label: string
  data_type: DataType
  unit?: string
  is_required: boolean
  position: number
  options?: string[]
}

export function useAddAttribute() {
  return useTypeMutation(
    ({ typeId, ...body }: AttributeInput & { typeId: string }) =>
      unwrap(inventoryApi.POST('/asset-types/{typeID}/attributes', { ...typePath(typeId), body })),
    false,
  )
}

export function useUpdateAttribute() {
  return useTypeMutation(
    ({ typeId, attrId, ...body }: Partial<Omit<AttributeInput, 'key' | 'options'>> & { typeId: string; attrId: string }) =>
      unwrap(
        inventoryApi.PATCH('/asset-types/{typeID}/attributes/{attributeID}', { ...attrPath(typeId, attrId), body }),
      ),
    false,
  )
}

export function useRemoveAttribute() {
  return useTypeMutation(({ typeId, attrId }: { typeId: string; attrId: string }) =>
    unwrap(inventoryApi.DELETE('/asset-types/{typeID}/attributes/{attributeID}', attrPath(typeId, attrId))),
  )
}

export function useAddOption() {
  return useTypeMutation(
    ({ typeId, attrId, ...body }: { typeId: string; attrId: string; label: string; position?: number }) =>
      unwrap(
        inventoryApi.POST('/asset-types/{typeID}/attributes/{attributeID}/options', { ...attrPath(typeId, attrId), body }),
      ),
  )
}

export function useUpdateOption() {
  return useTypeMutation(
    ({
      typeId,
      attrId,
      optionId,
      ...body
    }: {
      typeId: string
      attrId: string
      optionId: string
      label?: string
      position?: number
    }) =>
      unwrap(
        inventoryApi.PATCH('/asset-types/{typeID}/attributes/{attributeID}/options/{optionID}', {
          params: { path: { typeID: typeId, attributeID: attrId, optionID: optionId } },
          body,
        }),
      ),
  )
}

export function useRemoveOption() {
  return useTypeMutation(({ typeId, attrId, optionId }: { typeId: string; attrId: string; optionId: string }) =>
    unwrap(
      inventoryApi.DELETE('/asset-types/{typeID}/attributes/{attributeID}/options/{optionID}', {
        params: { path: { typeID: typeId, attributeID: attrId, optionID: optionId } },
      }),
    ),
  )
}
