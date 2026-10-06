import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { inventoryApi } from '@/lib/api/client'
import type { ExportLayout, ExportRequest } from '@/lib/api/types'
import { fileNameFrom, saveBlob } from '@/lib/download'
import { unwrap } from '@/lib/errors'

export const exportProfileKeys = { all: ['export-profiles'] as const }

export function useExportProfiles(enabled = true) {
  return useQuery({
    queryKey: exportProfileKeys.all,
    queryFn: async () => (await unwrap(inventoryApi.GET('/export-profiles'))).items,
    enabled,
  })
}

// toast: false khi form tự hiện lỗi
function useProfileMutation<V, R>(fn: (v: V) => Promise<R>, toast = true) {
  const qc = useQueryClient()
  return useMutation({ mutationFn: fn, meta: { toast }, onSuccess: () => qc.invalidateQueries({ queryKey: exportProfileKeys.all }) })
}

export function useCreateExportProfile() {
  return useProfileMutation(
    (body: { name: string; shared: boolean; layout: ExportLayout }) => unwrap(inventoryApi.POST('/export-profiles', { body })),
    false,
  )
}

export function useUpdateExportProfile() {
  return useProfileMutation(({ id, ...body }: { id: string; version: number; name?: string; shared?: boolean; layout?: ExportLayout }) =>
    unwrap(inventoryApi.PATCH('/export-profiles/{profileID}', { params: { path: { profileID: id } }, body })),
  )
}

export function useDeleteExportProfile() {
  return useProfileMutation((id: string) => unwrap(inventoryApi.DELETE('/export-profiles/{profileID}', { params: { path: { profileID: id } } })))
}

// exportAssets tải file và lưu; trả tên file và các cột bị bỏ.
// Lỗi (vd 422 quá số dòng) vẫn là problem JSON nên unwrap ném ApiError như thường.
export async function exportAssets(body: ExportRequest, fallbackName: string): Promise<{ name: string; skipped: string[] }> {
  const res = await inventoryApi.POST('/assets/export', { body, parseAs: 'blob' })
  const blob = (await unwrap(Promise.resolve(res))) as unknown as Blob
  const name = fileNameFrom(res.response.headers.get('Content-Disposition'), fallbackName)
  saveBlob(blob, name)
  const skipped = (res.response.headers.get('X-Export-Skipped-Columns') ?? '').split(',').filter(Boolean)
  return { name, skipped }
}
