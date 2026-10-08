// Tải trước dữ liệu của trang ngay khi phiên sẵn sàng, song song với tải code của trang
// (route lazy) thay vì đợi trang dựng xong mới gọi API. Dùng đúng khoá query của trang và
// sidebar (assetTypesQuery, statusesQuery, assetListQuery) nên trang lấy kết quả có sẵn.
import type { RouteLocationNormalized } from 'vue-router'
import { assetTypesQuery } from '@/features/asset-types/api'
import { assetListQuery } from '@/features/assets/api'
import { parseAssetQuery, toApiParams } from '@/features/assets/listQuery'
import { statusesQuery } from '@/features/statuses/api'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { pageSizeFor, type Prefs, usePreferences } from '@/lib/preferences'
import { queryClient } from '@/lib/query'

// assetListParams: tham số danh sách tài sản mà AssetsPage sẽ gọi cho route này (cùng URL,
// cùng số dòng đã lưu cho bảng); không phải trang danh sách thì undefined
export function assetListParams(to: RouteLocationNormalized, prefs: Prefs) {
  if (to.name !== 'assets' && to.name !== 'type-assets') return undefined
  const typeId = typeof to.params.typeId === 'string' ? to.params.typeId : undefined
  const state = { ...parseAssetQuery(to.query), typeId }
  return toApiParams(state, pageSizeFor(prefs, `assets:${typeId ?? 'all'}`))
}

// prefetchRoute: gọi sau khi phiên đã sẵn sàng (cần token); không đợi, lỗi để trang tự xử lý
export function prefetchRoute(to: RouteLocationNormalized) {
  const session = useSession()
  if (!session.signedIn || !session.can(Perm.AssetRead)) return
  const prefetch = (q: { queryKey: readonly unknown[]; queryFn: () => Promise<unknown> }) =>
    void queryClient.prefetchQuery({ ...q, meta: { toast: false } })
  // sidebar của mọi trang trong app: loại tài sản kèm số lượng
  prefetch(assetTypesQuery(false, true))
  const params = assetListParams(to, usePreferences().prefs)
  if (params) {
    prefetch(assetListQuery(params))
    prefetch(statusesQuery(true, false))
  }
}
