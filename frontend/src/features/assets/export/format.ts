// Ô của bản xem trước, cùng luật định dạng với server (spreadsheet/format.go)
import type { AssetListItem, ExportLayout } from '@/lib/api/types'
import { attrKey, type TypeInfo } from './layout'

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
const KIND: Record<string, string> = { available: 'Available', in_use: 'In use', unavailable: 'Unavailable', retired: 'Retired' }

export function formatDate(iso: string | undefined | null, f: ExportLayout['date_format']): string {
  if (!iso) return ''
  const [y, m, d] = iso.slice(0, 10).split('-')
  if (f === 'yyyy-mm-dd') return `${y}-${m}-${d}`
  if (f === 'd mmm yyyy') return `${Number(d)} ${MONTHS[Number(m) - 1]} ${y}`
  return `${d}/${m}/${y}`
}

export function formatCell(row: AssetListItem, field: string, l: ExportLayout, _types: TypeInfo[]): { text: string; align?: 'right' | 'center' } {
  switch (field) {
    case 'tag':
      return { text: row.tag }
    case 'name':
      return { text: row.name }
    case 'description':
      return { text: '' } // danh sách tài sản không trả mô tả
    case 'type':
      return { text: row.asset_type_name }
    case 'status':
      return { text: l.status_as === 'kind' ? KIND[row.status_kind] : row.status_name }
    case 'purchase_date':
      return { text: formatDate(row.purchase_date, l.date_format), align: 'right' }
    case 'updated_at':
      return { text: `${formatDate(row.updated_at, l.date_format)} ${new Date(row.updated_at).toTimeString().slice(0, 5)}`, align: 'right' }
  }
  const key = attrKey(field)
  const v = row.attributes?.find((a) => a.key === key)
  if (!v || v.value === null || v.value === undefined) return { text: '' }
  switch (v.data_type) {
    case 'number':
      return { text: l.unit_in === 'cell' && v.unit ? `${v.value} ${v.unit}` : String(v.value), align: 'right' }
    case 'date':
      return { text: formatDate(String(v.value), l.date_format), align: 'right' }
    case 'boolean':
      return { text: l.bool_style === 'check' ? (v.value ? '✓' : '–') : v.value ? 'Yes' : 'No', align: 'center' }
    case 'select':
      return { text: v.option_label ?? '' }
  }
  return { text: String(v.value) }
}
