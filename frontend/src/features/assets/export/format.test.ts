import { describe, expect, it } from 'vitest'
import type { AssetListItem } from '@/lib/api/types'
import { formatCell } from './format'
import { defaultReportLayout, type TypeInfo } from './layout'

const laptop: TypeInfo = { id: 'L', name: 'Laptop', code: 'LAPTOP', attributes: [{ key: 'ram_gb', label: 'RAM', data_type: 'number', unit: 'GB' }] }
const row = {
  id: '1', tag: 'LAP-1', name: 'ThinkPad', asset_type_id: 'L', asset_type_name: 'Laptop', status_id: 's', status_name: 'On loan',
  status_kind: 'in_use', purchase_date: '2025-03-14', version: 1, updated_at: '2026-10-06T00:00:00Z',
  attributes: [
    { key: 'ram_gb', label: 'RAM', data_type: 'number', unit: 'GB', value: 16 },
    { key: 'touch', label: 'Touch', data_type: 'boolean', value: true },
  ],
} as AssetListItem

describe('formatCell', () => {
  it('formats dates, numbers, booleans and status like the server', () => {
    const l = defaultReportLayout()
    expect(formatCell(row, 'purchase_date', l, [laptop]).text).toBe('14/03/2025')
    expect(formatCell(row, 'purchase_date', { ...l, date_format: 'd mmm yyyy' }, [laptop]).text).toBe('14 Mar 2025')
    expect(formatCell(row, 'attr:ram_gb', l, [laptop])).toEqual({ text: '16', align: 'right' })
    expect(formatCell(row, 'attr:ram_gb', { ...l, unit_in: 'cell' }, [laptop]).text).toBe('16 GB')
    expect(formatCell(row, 'attr:touch', l, [laptop]).text).toBe('Yes')
    expect(formatCell(row, 'attr:touch', { ...l, bool_style: 'check' }, [laptop]).text).toBe('✓')
    expect(formatCell(row, 'status', l, [laptop]).text).toBe('On loan')
    expect(formatCell(row, 'status', { ...l, status_as: 'kind' }, [laptop]).text).toBe('In use')
    expect(formatCell(row, 'attr:missing', l, [laptop]).text).toBe('')
  })
})
