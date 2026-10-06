// Bố cục báo cáo ở giao diện: mặc định, danh sách trường, tiêu đề mặc định, dựng sheet
// cho bản xem trước. Khớp backend (inventory/domain/export.go).
import type { AssetListItem, DataType, ExportLayout } from '@/lib/api/types'

export interface TypeAttr {
  key: string
  label: string
  data_type: DataType
  unit?: string
}
export interface TypeInfo {
  id: string
  name: string
  code: string
  attributes: TypeAttr[] // đang dùng, theo thứ tự hiển thị
}
export interface FieldOption {
  field: string
  label: string
  unit?: string
  types: string[] // tên loại có thuộc tính này; rỗng là trường chung
}
export interface EditorColumn {
  field: string
  header: string
  width: number // 0: tự động
  include: boolean
}
export interface PreviewColumn {
  field: string
  header: string
}
export interface PreviewSheet {
  name: string
  columns: PreviewColumn[]
  rows: AssetListItem[]
}

export const COMMON_FIELDS: { field: string; label: string }[] = [
  { field: 'tag', label: 'Tag' },
  { field: 'name', label: 'Name' },
  { field: 'description', label: 'Description' },
  { field: 'type', label: 'Type' },
  { field: 'status', label: 'Status' },
  { field: 'purchase_date', label: 'Purchase date' },
  { field: 'updated_at', label: 'Updated' },
]

export const attrKey = (field: string) => (field.startsWith('attr:') ? field.slice(5) : null)

export function defaultReportLayout(): ExportLayout {
  return {
    columns: ['tag', 'name', 'type', 'status', 'purchase_date'].map((field) => ({ field })),
    sheets: 'single',
    each_type_attrs: false,
    sheet_name: 'Assets',
    title_row: false,
    summary: false,
    header: 'bold',
    freeze: true,
    filter: true,
    stripes: false,
    date_format: 'dd/mm/yyyy',
    bool_style: 'yes_no',
    status_as: 'name',
    unit_in: 'header',
    sort: '',
  }
}

export function fieldOptions(types: TypeInfo[]): FieldOption[] {
  const out: FieldOption[] = COMMON_FIELDS.map((c) => ({ ...c, types: [] }))
  const byKey = new Map<string, FieldOption>()
  for (const t of types) {
    for (const a of t.attributes) {
      const f = byKey.get(a.key)
      if (f) f.types.push(t.name)
      else {
        const opt = { field: `attr:${a.key}`, label: a.label, unit: a.unit, types: [t.name] }
        byKey.set(a.key, opt)
        out.push(opt)
      }
    }
  }
  return out
}

export function defaultHeader(field: string, layout: ExportLayout, options: FieldOption[]): string {
  const o = options.find((x) => x.field === field)
  const label = o?.label ?? attrKey(field) ?? field
  return o?.unit && layout.unit_in === 'header' ? `${label} (${o.unit})` : label
}

// editorColumns: cột của bố cục (đã chọn, đúng thứ tự) rồi các trường còn lại (chưa chọn)
export function editorColumns(layout: ExportLayout, options: FieldOption[]): EditorColumn[] {
  const chosen = layout.columns.map((c) => ({ field: c.field, header: c.header ?? '', width: c.width ?? 0, include: true }))
  const rest = options.filter((o) => !chosen.some((c) => c.field === o.field)).map((o) => ({ field: o.field, header: '', width: 0, include: false }))
  return [...chosen, ...rest]
}

// withColumns: bố cục với các cột đang chọn của trình sửa cột
export function withColumns(layout: ExportLayout, cols: EditorColumn[]): ExportLayout {
  return { ...layout, columns: cols.filter((c) => c.include).map((c) => ({ field: c.field, header: c.header.trim() || undefined, width: c.width })) }
}

function sheetColumns(layout: ExportLayout, types: TypeInfo[], options: FieldOption[]): PreviewColumn[] {
  const has = (key: string) => types.some((t) => t.attributes.some((a) => a.key === key))
  const out: PreviewColumn[] = []
  for (const c of layout.columns) {
    const key = attrKey(c.field)
    if (key && !has(key)) continue
    out.push({ field: c.field, header: c.header?.trim() || defaultHeader(c.field, layout, options) })
  }
  if (layout.sheets === 'per_type' && layout.each_type_attrs) {
    for (const t of types) {
      for (const a of t.attributes) {
        const f = `attr:${a.key}`
        if (!out.some((c) => c.field === f)) out.push({ field: f, header: defaultHeader(f, layout, options) })
      }
    }
  }
  return out
}

export function previewSheets(layout: ExportLayout, rows: AssetListItem[], types: TypeInfo[]): PreviewSheet[] {
  const options = fieldOptions(types)
  if (layout.sheets === 'single') {
    return [{ name: layout.sheet_name || 'Assets', columns: sheetColumns(layout, types, options), rows }]
  }
  return [...types]
    .sort((a, b) => a.name.localeCompare(b.name))
    .map((t) => ({ name: t.name, columns: sheetColumns(layout, [t], options), rows: rows.filter((r) => r.asset_type_id === t.id) }))
    .filter((s) => s.rows.length)
}

export function skippedKeys(layout: ExportLayout, types: TypeInfo[]): string[] {
  return layout.columns
    .map((c) => attrKey(c.field))
    .filter((k): k is string => !!k && !types.some((t) => t.attributes.some((a) => a.key === k)))
}

// normalizeLayout: chuỗi so sánh để biết bố cục đã đổi so với profile chưa. Dựng object
// với thứ tự khoá cố định nên bố cục từ server và bố cục dựng lại ở máy so sánh đúng
export function normalizeLayout(l: ExportLayout): string {
  return JSON.stringify({
    columns: l.columns.map((c) => ({ field: c.field, header: c.header?.trim() || '', width: c.width ?? 0 })),
    sheets: l.sheets,
    each_type_attrs: !!l.each_type_attrs,
    sheet_name: l.sheet_name,
    title_row: l.title_row,
    summary: l.summary,
    header: l.header,
    freeze: l.freeze,
    filter: l.filter,
    stripes: l.stripes,
    date_format: l.date_format,
    bool_style: l.bool_style,
    status_as: l.status_as,
    unit_in: l.unit_in,
    sort: l.sort ?? '',
  })
}
