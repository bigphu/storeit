// Phạm vi "mọi tài sản" cho xuất và bản xem trước khi đứng ở trang profile (không có bộ
// lọc của danh sách nào)
import { computed } from 'vue'
import { useAssetTypes } from '@/features/asset-types/api'
import type { ExportScope } from '@/features/assets/export/usePreviewData'

export function useAllAssetsScope() {
  const { data: types } = useAssetTypes(false, true)
  return computed<ExportScope>(() => ({
    filters: {},
    label: 'All assets',
    count: (types.value ?? []).reduce((n, t) => n + (t.asset_count ?? 0), 0),
    typeIds: (types.value ?? []).filter((t) => (t.asset_count ?? 0) > 0).map((t) => t.id),
    rows: [],
    selection: false,
  }))
}
