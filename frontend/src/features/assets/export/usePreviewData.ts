// Dữ liệu cho bản xem trước: định nghĩa thuộc tính của MỌI loại trong phạm vi (để chọn cột
// và biết cột nào còn) và tối đa 20 dòng mỗi loại, chỉ cho vài loại đầu (danh sách chỉ có
// giá trị thuộc tính khi lọc theo một loại)
import { useQueries } from '@tanstack/vue-query'
import { computed } from 'vue'
import { typeKeys } from '@/features/asset-types/api'
import { inventoryApi } from '@/lib/api/client'
import type { AssetListItem, ExportFilters } from '@/lib/api/types'
import { unwrap } from '@/lib/errors'
import type { TypeInfo } from './layout'

export interface ExportScope {
  filters: ExportFilters
  label: string
  count: number
  typeIds: string[]
  rows: AssetListItem[] // dòng đang thấy (dùng khi xem trước phần đã chọn)
  selection: boolean
}

// giới hạn chỉ áp cho truy vấn dòng xem trước, không áp cho định nghĩa loại
const MAX_PREVIEW_TYPES = 8

export function usePreviewData(scope: () => ExportScope, enabled: () => boolean) {
  const typeIds = computed(() => scope().typeIds)
  const previewIds = computed(() => typeIds.value.slice(0, MAX_PREVIEW_TYPES))
  // cùng khoá với useAssetType nên dùng chung cache với trang loại
  const typeQueries = useQueries({
    queries: computed(() =>
      typeIds.value.map((id) => ({
        queryKey: typeKeys.one(id),
        queryFn: () => unwrap(inventoryApi.GET('/asset-types/{typeID}', { params: { path: { typeID: id } } })),
        enabled: enabled(),
      })),
    ),
  })
  const rowQueries = useQueries({
    queries: computed(() =>
      previewIds.value.map((id) => {
        const f = scope().filters
        return {
          queryKey: ['export-preview', id, f],
          queryFn: async () =>
            (
              await unwrap(
                inventoryApi.GET('/assets', {
                  params: {
                    query: {
                      q: f.q,
                      type_id: id,
                      status_id: f.status_id,
                      status_kind: f.status_kind,
                      include_retired: f.include_retired,
                      attr: f.type_id ? f.attr : undefined,
                      sort: f.sort,
                      page: 1,
                      page_size: 20,
                    },
                  },
                }),
              )
            ).items,
          enabled: enabled() && !scope().selection,
        }
      }),
    ),
  })
  const types = computed<TypeInfo[]>(() =>
    typeQueries.value
      .map((q) => q.data)
      .filter((t): t is NonNullable<typeof t> => !!t)
      .map((t) => ({
        id: t.id,
        name: t.name,
        code: t.code,
        attributes: t.attributes.filter((a) => !a.removed).map((a) => ({ key: a.key, label: a.label, data_type: a.data_type, unit: a.unit ?? undefined })),
      })),
  )
  const rows = computed<AssetListItem[]>(() => (scope().selection ? scope().rows : rowQueries.value.flatMap((q) => q.data ?? [])))
  return { types, rows }
}
