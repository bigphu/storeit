// Giá trị thuộc tính tuỳ chỉnh: giữa API (AttributeValue, body "attributes") và form.
import type { AssetDetail, AttributeValue, DataType } from '@/lib/api/types'
import { formatDate, fromDateString, toDateString } from '@/lib/dates'
import type { AssetBody } from './api'

// Giá trị trong form theo kiểu: text string, number number|null, date Date|null,
// boolean 'true'|'false'|null (null là chưa đặt), select id option|null
export type FormValue = string | number | Date | null
export type FormValues = Record<string, FormValue>

interface AttrLike {
  key: string
  data_type: DataType
}

function emptyValue(t: DataType): FormValue {
  return t === 'text' ? '' : null
}

export function fromApiValues(attrs: readonly AttrLike[], values: readonly AttributeValue[]): FormValues {
  const byKey = new Map(values.map((v) => [v.key, v.value]))
  const out: FormValues = {}
  for (const a of attrs) {
    const v = byKey.get(a.key)
    if (v === null || v === undefined) {
      out[a.key] = emptyValue(a.data_type)
      continue
    }
    switch (a.data_type) {
      case 'number':
        out[a.key] = Number(v)
        break
      case 'date':
        out[a.key] = fromDateString(String(v))
        break
      case 'boolean':
        out[a.key] = v ? 'true' : 'false'
        break
      default:
        out[a.key] = String(v)
    }
  }
  return out
}

// toApiValues: body "attributes"; giá trị trống bỏ đi (PUT: thuộc tính thiếu là xoá giá trị)
export function toApiValues(attrs: readonly AttrLike[], form: FormValues): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const a of attrs) {
    const v = form[a.key]
    if (v === null || v === undefined) continue
    switch (a.data_type) {
      case 'text': {
        const s = String(v).trim()
        if (s) out[a.key] = s
        break
      }
      case 'number':
        out[a.key] = Number(v)
        break
      case 'date':
        out[a.key] = toDateString(v as Date)
        break
      case 'boolean':
        out[a.key] = v === 'true'
        break
      case 'select':
        out[a.key] = String(v)
        break
    }
  }
  return out
}

// formatValue: hiện một giá trị trong bảng hay trang chi tiết
export function formatValue(v: AttributeValue | undefined): string {
  if (!v || v.value === null || v.value === undefined) return '—'
  switch (v.data_type) {
    case 'number':
      return v.unit ? `${v.value} ${v.unit}` : String(v.value)
    case 'date':
      return formatDate(String(v.value))
    case 'boolean':
      return v.value ? 'Yes' : 'No'
    case 'select':
      return v.option_removed ? `${v.option_label} (removed)` : (v.option_label ?? String(v.value))
    default:
      return String(v.value)
  }
}

// assetBodyOf: body PUT dựng lại từ tài sản đã lưu (Undo của "Save"), cùng định dạng form gửi
export function assetBodyOf(a: AssetDetail): AssetBody {
  return {
    name: a.name,
    description: a.description,
    asset_type_id: a.asset_type.id,
    status_id: a.status.id,
    location_id: a.location_id,
    holder_member_id: a.holder_member_id,
    purchase_date: a.purchase_date,
    attributes: toApiValues(a.attributes, fromApiValues(a.attributes, a.attributes)),
  }
}

// attrText: giá trị thuộc tính của dòng danh sách thành chữ để sửa tại chỗ (số, ngày
// YYYY-MM-DD, 'true'/'false', id option, chữ); trống là ''
export function attrText(v: AttributeValue | undefined): string {
  if (!v || v.value === null || v.value === undefined) return ''
  return String(v.value)
}

// formValueFrom: chữ vừa sửa thành giá trị form đúng kiểu (toApiValues đổi tiếp sang API)
export function formValueFrom(type: DataType, text: string): FormValue {
  const t = text.trim()
  switch (type) {
    case 'number':
      return t === '' ? null : Number(t)
    case 'date':
      return fromDateString(t)
    case 'boolean':
    case 'select':
      return t === '' ? null : t
    default:
      return text
  }
}
