// Điều hướng theo loại tài sản: loại đang mở (phạm vi của sidebar) và nơi đến khi
// chọn một loại khác (cùng mục: Assets hay Settings; mỗi loại mở lại view đã nhớ)
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useAsset } from '@/features/assets/api'
import { useTabs } from './tabs/useTabs'
import { useListContext } from '@/features/assets/listContext'
import { type ListLocation, listLocation, switchType } from '@/features/assets/listQuery'

const LIST_ROUTES = ['assets', 'type-assets']
const ASSET_ROUTES = ['asset', 'asset-edit']

export function useTypeNav() {
  const route = useRoute()
  const listContext = useListContext()
  const tabs = useTabs()

  // trang của một tài sản thuộc phạm vi loại của nó (lấy từ cache của trang)
  const assetId = computed(() => (ASSET_ROUTES.includes(String(route.name)) ? String(route.params.id) : undefined))
  const { data: asset } = useAsset(assetId)

  const scopeTypeId = computed<string | undefined>(() => {
    if (typeof route.params.typeId === 'string') return route.params.typeId
    if (assetId.value && asset.value?.id === assetId.value) return asset.value.asset_type.id
    return undefined
  })

  // Đang xem một danh sách thì tìm kiếm, status, "include retired" đi theo sang loại mới
  function listFor(typeId: string | undefined): ListLocation {
    const views = listContext.views
    const current = LIST_ROUTES.includes(String(route.name)) ? listContext.ctxFor(tabs.activeId)?.state : undefined
    if (current) return listLocation(switchType(current, typeId, views).state)
    const v = views[typeId ?? '']
    return listLocation({ q: '', includeRetired: false, typeId, filters: v?.filters ?? [], fields: [], sort: v?.sort, page: v?.page ?? 1 })
  }

  // locationFor: chọn loại trong bộ chọn: giữ mục đang mở
  function locationFor(typeId: string): ListLocation | string {
    return route.name === 'type-settings' ? `/types/${typeId}/settings` : listFor(typeId)
  }

  return { scopeTypeId, listFor, locationFor }
}
