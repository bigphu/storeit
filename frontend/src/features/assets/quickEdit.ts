// Sửa nhanh tài sản từ danh sách (ô sửa tại chỗ, ngăn kéo): PUT thay toàn bộ nên dựng body
// từ tài sản đọc mới nhất rồi chỉ đổi phần vừa sửa
import type { AssetDetail } from '@/lib/api/types'
import type { AssetBody } from './api'
import { assetBodyOf } from './values'

export type AssetQuickChange = Partial<Pick<AssetBody, 'name' | 'status_id' | 'purchase_date' | 'description' | 'attributes'>>

export function quickAssetBody(a: AssetDetail, change: AssetQuickChange): AssetBody {
  return { ...assetBodyOf(a), ...change }
}
