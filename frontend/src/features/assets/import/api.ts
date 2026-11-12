import { useMutation } from '@tanstack/vue-query'
import { inventoryApi } from '@/lib/api/client'
import { fileNameFrom, saveBlob } from '@/lib/download'
import { unwrap } from '@/lib/errors'

// postImport gửi nguyên file (octet-stream); preview không ghi gì, commit tạo / sửa từng dòng.
// Lỗi của cả file (sai mẫu, quá lớn, đã import) là problem JSON nên unwrap ném ApiError
export function postImport(file: File, mode: 'preview' | 'commit') {
  return unwrap(
    inventoryApi.POST('/assets/import', {
      params: { query: { mode, file_name: file.name } },
      body: file as unknown as string,
      bodySerializer: (b) => b as unknown as BodyInit,
      headers: { 'Content-Type': 'application/octet-stream' },
    }),
  )
}

// toast: false vì trang tự hiện lỗi của file ngay trong khung tải lên
export function useImportPreview() {
  return useMutation({ mutationFn: (file: File) => postImport(file, 'preview'), meta: { toast: false } })
}
export function useImportCommit() {
  return useMutation({ mutationFn: (file: File) => postImport(file, 'commit'), meta: { toast: false } })
}

export async function downloadImportTemplate(): Promise<string> {
  const res = await inventoryApi.GET('/assets/import/template', { parseAs: 'blob' })
  const blob = (await unwrap(Promise.resolve(res))) as unknown as Blob
  const name = fileNameFrom(res.response.headers.get('Content-Disposition'), 'storeit-import-template.xlsx')
  saveBlob(blob, name)
  return name
}
