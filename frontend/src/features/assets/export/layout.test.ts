import { describe, expect, it } from 'vitest'
import { reactive, readonly } from 'vue'
import type { AssetListItem, ExportLayout } from '@/lib/api/types'
import { cleanSheetName, cloneLayout, excludeFields, includeAttributes, profilePatch, dataPreviewSheets, defaultHeader, exportFileName, defaultReportLayout, editorColumns, editorRowsFor, type EditorColumn, fieldOptions, reorderVisible, normalizeLayout, previewSheets, sheetNameInput, skippedKeys, type TypeInfo, withColumns } from './layout'

const laptop: TypeInfo = { id: 'L', name: 'Laptop', code: 'LAPTOP', attributes: [{ key: 'ram_gb', label: 'RAM', data_type: 'number', unit: 'GB' }, { key: 'cpu', label: 'CPU', data_type: 'text' }] }
const phone: TypeInfo = { id: 'P', name: 'Phone', code: 'PHONE', attributes: [{ key: 'imei', label: 'IMEI', data_type: 'text' }] }
const row = (tag: string, typeId: string) => ({ id: tag, tag, name: tag, asset_type_id: typeId, asset_type_name: typeId, status_id: 's', status_name: 'Available', status_kind: 'available', version: 1, updated_at: '2026-10-06T00:00:00Z' }) as AssetListItem

describe('layout', () => {
  it('lists common fields then attributes once', () => {
    const opts = fieldOptions([laptop, phone])
    expect(opts.map((o) => o.field)).toEqual(['tag', 'name', 'description', 'type', 'status', 'purchase_date', 'updated_at', 'attr:ram_gb', 'attr:cpu', 'attr:imei'])
  })

  it('puts the unit in the default header when asked', () => {
    const l = defaultReportLayout()
    const opts = fieldOptions([laptop])
    expect(defaultHeader('attr:ram_gb', l, opts)).toBe('RAM (GB)')
    expect(defaultHeader('attr:ram_gb', { ...l, unit_in: 'cell' }, opts)).toBe('RAM')
  })

  it('builds per-type sheets with only that type’s attribute columns', () => {
    const l = { ...defaultReportLayout(), sheets: 'per_type' as const, columns: [{ field: 'tag' }, { field: 'attr:imei' }] }
    const sheets = previewSheets(l, [row('L1', 'L'), row('P1', 'P')], [laptop, phone])
    expect(sheets.map((s) => s.name)).toEqual(['Laptop', 'Phone'])
    expect(sheets[0].columns.map((c) => c.field)).toEqual(['tag'])
    expect(sheets[1].columns.map((c) => c.field)).toEqual(['tag', 'attr:imei'])
    const each = previewSheets({ ...l, each_type_attrs: true }, [row('L1', 'L')], [laptop])
    expect(each[0].columns.map((c) => c.field)).toEqual(['tag', 'attr:ram_gb', 'attr:cpu'])
  })

  it('reports attribute keys no type has', () => {
    const l = { ...defaultReportLayout(), columns: [{ field: 'tag' }, { field: 'attr:gone' }] }
    expect(skippedKeys(l, [laptop])).toEqual(['gone'])
  })

  it('normalizes layouts independent of key order', () => {
    const a = defaultReportLayout()
    const reversed = Object.fromEntries(Object.entries(a).reverse()) as typeof a
    expect(Object.keys(reversed)).not.toEqual(Object.keys(a))
    expect(normalizeLayout(reversed)).toBe(normalizeLayout(a))
    const withHeader = { ...a, columns: [{ header: ' Asset tag ', field: 'tag' }, ...a.columns.slice(1)] }
    const sameHeader = { ...a, columns: [{ field: 'tag', header: 'Asset tag' }, ...a.columns.slice(1)] }
    expect(normalizeLayout(withHeader)).toBe(normalizeLayout(sameHeader))
    expect(normalizeLayout(withHeader)).not.toBe(normalizeLayout(a))
  })

  it('round-trips a server-shaped layout through the editor columns', () => {
    const a = defaultReportLayout()
    const server = { ...a, columns: a.columns.map((c) => ({ field: c.field, header: '', width: 0 })) }
    const opts = fieldOptions([laptop])
    expect(normalizeLayout(withColumns(server, editorColumns(server, opts)))).toBe(normalizeLayout(server))
    const wide = { ...server, columns: server.columns.map((c, i) => (i === 1 ? { ...c, width: 20 } : c)) }
    const back = withColumns(wide, editorColumns(wide, opts))
    expect(back.columns[1].width).toBe(20)
    expect(normalizeLayout(back)).toBe(normalizeLayout(wide))
  })
})

describe('sheet names', () => {
  it('replaces characters Excel forbids while typing, keeps apostrophes', () => {
    expect(sheetNameInput('Laptops 06/10/2026')).toBe('Laptops 06-10-2026')
    expect(sheetNameInput('a[b]c:d*e?f\\g')).toBe('a-b-c-d-e-f-g')
    expect(sheetNameInput("it's")).toBe("it's")
  })

  it('drops leading and trailing apostrophes before saving', () => {
    expect(cleanSheetName("'Quoted'")).toBe('Quoted')
    expect(cleanSheetName("  it's: ok'")).toBe("it's- ok")
  })
})

describe('data export preview', () => {
  it('has one sheet per type named by code, with field keys as headers', () => {
    const sheets = dataPreviewSheets([row('P1', 'P'), row('L1', 'L')], [phone, laptop])
    expect(sheets.map((s) => s.name)).toEqual(['LAPTOP', 'PHONE'])
    expect(sheets[0].columns.map((c) => c.header)).toEqual(['tag', 'name', 'description', 'status', 'purchase_date', 'attr:ram_gb', 'attr:cpu'])
    expect(sheets[0].rows.map((r) => r.tag)).toEqual(['L1'])
  })
})

describe('exportFileName', () => {
  it('names files like the server', () => {
    expect(exportFileName('data', undefined, '2026-10-06')).toBe('storeit-assets-2026-10-06.xlsx')
    expect(exportFileName('report', undefined, '2026-10-06')).toBe('storeit-report-2026-10-06.xlsx')
    expect(exportFileName('report', 'Kiểm kê quý 3', '2026-10-06')).toBe('kiểm-kê-quý-3-2026-10-06.xlsx')
    expect(exportFileName('report', '!!!', '2026-10-06')).toBe('storeit-report-2026-10-06.xlsx')
  })
})

describe('cloneLayout', () => {
  // profile lấy từ Vue Query là proxy readonly(reactive): structuredClone ném lỗi với nó
  it('copies a layout held in a Vue Query proxy', () => {
    const l = { ...defaultReportLayout(), columns: [{ field: 'tag', header: 'Mã' }] }
    // Vue Query khai kiểu dữ liệu là ExportLayout thường, dù thực tế là proxy readonly
    const proxied = readonly(reactive({ layout: l })).layout as ExportLayout
    const copy = cloneLayout(proxied)
    expect(copy).toEqual(l)
    copy.columns[0].header = 'changed'
    expect(l.columns[0].header).toBe('Mã')
  })
})

describe('each type attributes in the column editor', () => {
  const cols = [
    { field: 'tag', header: '', width: 0, include: true },
    { field: 'attr:ram_gb', header: '', width: 0, include: true },
    { field: 'attr:cpu', header: '', width: 0, include: false },
    { field: 'attr:imei', header: '', width: 0, include: false },
    { field: 'attr:gone', header: '', width: 0, include: false },
  ]
  const opts = fieldOptions([laptop, phone])

  it('ticks every available attribute column and reports which it ticked', () => {
    const { columns, added } = includeAttributes(cols, opts)
    expect(columns.filter((c) => c.include).map((c) => c.field)).toEqual(['tag', 'attr:ram_gb', 'attr:cpu', 'attr:imei'])
    expect(added).toEqual(['attr:cpu', 'attr:imei'])
  })

  it('unticks only the columns it ticked', () => {
    const { columns, added } = includeAttributes(cols, opts)
    const back = excludeFields(columns, added)
    expect(back.filter((c) => c.include).map((c) => c.field)).toEqual(['tag', 'attr:ram_gb'])
  })
})

describe('profilePatch', () => {
  const p = { name: 'Laptop chi tiết', shared: false }
  const l = defaultReportLayout()

  it('is null when nothing changed (Save stays off)', () => {
    expect(profilePatch(p, { name: ' Laptop chi tiết ', shared: false }, null)).toBeNull()
  })

  it('sends only what changed', () => {
    expect(profilePatch(p, { name: 'Laptop 2026', shared: false }, null)).toEqual({ name: 'Laptop 2026' })
    expect(profilePatch(p, { name: 'Laptop chi tiết', shared: true }, null)).toEqual({ shared: true })
    expect(profilePatch(p, { name: 'Laptop chi tiết', shared: false }, l)).toEqual({ layout: l })
  })

  it('treats a blank name as no rename', () => {
    expect(profilePatch(p, { name: '   ', shared: false }, null)).toBeNull()
  })
})

describe('v1.0.2 report defaults and scoped column editor', () => {
  const laptop: TypeInfo = { id: 't1', name: 'Laptop', code: 'LAP', attributes: [{ key: 'ram_gb', label: 'RAM', data_type: 'number' }] }
  const monitor: TypeInfo = { id: 't2', name: 'Monitor', code: 'MON', attributes: [{ key: 'size_in', label: 'Size', data_type: 'number' }] }
  const col = (field: string, include = true): EditorColumn => ({ field, header: '', width: 0, include })
  const cols = [col('tag'), col('attr:ram_gb'), col('name'), col('attr:size_in', false), col('attr:gone')]

  it('starts new reports with one sheet per type', () => {
    expect(defaultReportLayout().sheets).toBe('per_type')
  })

  it('tags each per-type preview sheet with its type', () => {
    const rows = [
      { asset_type_id: 't1' },
      { asset_type_id: 't2' },
    ] as unknown as Parameters<typeof previewSheets>[1]
    const sheets = previewSheets(defaultReportLayout(), rows, [laptop, monitor])
    expect(sheets.map((s) => [s.name, s.typeId])).toEqual([
      ['Laptop', 't1'],
      ['Monitor', 't2'],
    ])
  })

  it('shows built-in fields plus the scoped type attributes; null shows every row', () => {
    expect(editorRowsFor(cols, [laptop]).map((c) => c.field)).toEqual(['tag', 'attr:ram_gb', 'name'])
    expect(editorRowsFor(cols, [monitor]).map((c) => c.field)).toEqual(['tag', 'name', 'attr:size_in'])
    expect(editorRowsFor(cols, null)).toBe(cols)
  })

  it('reorders visible rows and keeps hidden rows in place', () => {
    // all: tag, attr:ram_gb, name, attr:size_in (ẩn), attr:gone (ẩn)
    const visible = editorRowsFor(cols, [laptop]) // tag, attr:ram_gb, name
    const reordered = [visible[2], visible[0], visible[1]] // name, tag, attr:ram_gb
    // ô của dòng hiện (0, 1, 2) được lấp theo thứ tự mới; ô 3, 4 giữ nguyên
    expect(reorderVisible(cols, reordered).map((c) => c.field)).toEqual(['name', 'tag', 'attr:ram_gb', 'attr:size_in', 'attr:gone'])
  })
})
