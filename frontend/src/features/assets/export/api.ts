import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { inventoryApi } from '@/lib/api/client'
import type { ExportFilters, ExportLayout, ExportRequest } from '@/lib/api/types'
import { fileNameFrom, saveBlob } from '@/lib/download'
import { unwrap } from '@/lib/errors'

export const exportProfileKeys = { all: ['export-profiles'] as const }

// previewListQuery: tham số GET /assets cho bản xem trước của một loại, cùng bộ lọc với file
// export, kể cả trường có sẵn và múi giờ của chúng. Thuộc tính chỉ áp khi bộ lọc có loại
export function previewListQuery(f: ExportFilters, typeId: string) {
  return {
    q: f.q,
    type_id: typeId,
    status_id: f.status_id,
    status_kind: f.status_kind,
    include_retired: f.include_retired,
    attr: f.type_id ? f.attr : undefined,
    field: f.field,
    tz: f.field?.length ? Intl.DateTimeFormat().resolvedOptions().timeZone : undefined,
    sort: f.sort,
    page: 1,
    page_size: 20,
  }
}

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

// toast: false khi form tự hiện lỗi (popover Save… của hộp thoại báo cáo)
export function useUpdateExportProfile(toast = true) {
  return useProfileMutation(
    ({ id, ...body }: { id: string; version: number; name?: string; shared?: boolean; layout?: ExportLayout }) =>
      unwrap(inventoryApi.PATCH('/export-profiles/{profileID}', { params: { path: { profileID: id } }, body })),
    toast,
  )
}

export function useDeleteExportProfile() {
  return useProfileMutation(
    (id: string) => unwrap(inventoryApi.DELETE('/export-profiles/{profileID}', { params: { path: { profileID: id } } })),
    false,
  )
}

// Undo của xoá profile (xoá mềm)
export function useRestoreExportProfile() {
  return useProfileMutation(
    (id: string) => unwrap(inventoryApi.POST('/export-profiles/{profileID}/restore', { params: { path: { profileID: id } } })),
    false,
  )
}

// exportAssets tải file và lưu; trả tên file và các cột bị bỏ.
// Lỗi (vd 422 quá số dòng) vẫn là problem JSON nên unwrap ném ApiError như thường.
export async function exportAssets(body: ExportRequest, fallbackName: string): Promise<{ name: string; skipped: string[] }> {
  // múi giờ của trình duyệt: giờ cập nhật, ngày ở tiêu đề và tên file theo giờ của người dùng
  const tz = Intl.DateTimeFormat().resolvedOptions().timeZone
  const res = await inventoryApi.POST('/assets/export', { body: { ...body, tz }, parseAs: 'blob' })
  const blob = (await unwrap(Promise.resolve(res))) as unknown as Blob
  const name = fileNameFrom(res.response.headers.get('Content-Disposition'), fallbackName)
  saveBlob(blob, name)
  const skipped = (res.response.headers.get('X-Export-Skipped-Columns') ?? '').split(',').filter(Boolean)
  return { name, skipped }
}
