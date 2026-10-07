// Sửa nhanh tài sản từ danh sách (ô sửa tại chỗ, ngăn kéo): PUT thay toàn bộ nên dựng body
// từ tài sản đọc mới nhất rồi chỉ đổi phần vừa sửa
import type { AssetDetail } from '@/lib/api/types'
import type { AssetBody } from './api'
import { assetBodyOf, type FormValues, fromApiValues, toApiValues } from './values'

export type AssetQuickChange = Partial<Pick<AssetBody, 'name' | 'status_id' | 'purchase_date' | 'description' | 'attributes'>>

// attrPatch: chỉ những thuộc tính vừa sửa (giá trị form); các thuộc tính khác giữ nguyên
export function quickAssetBody(a: AssetDetail, change: AssetQuickChange, attrPatch?: FormValues): AssetBody {
  const body = { ...assetBodyOf(a), ...change }
  if (attrPatch && Object.keys(attrPatch).length) {
    body.attributes = toApiValues(a.attributes, { ...fromApiValues(a.attributes, a.attributes), ...attrPatch })
  }
  return body
}
