import { describe, expect, it } from 'vitest'
import type { Attribute } from '@/lib/api/types'
import { attrFilterLabel, fieldFilterLabel, filterChips, OP_LABEL, removeChip } from './filterChips'
import { type AssetListState, parseAssetQuery } from './listQuery'

const attrs = [
  { id: 'a1', key: 'ram_gb', label: 'RAM', data_type: 'number', unit: 'GB', options: [] },
  {
    id: 'a2',
    key: 'os',
    label: 'OS',
    data_type: 'select',
    options: [
      { id: 'w', label: 'Windows' },
      { id: 'm', label: 'macOS' },
    ],
  },
  { id: 'a3', key: 'has_dock', label: 'Dock', data_type: 'boolean', options: [] },
] as unknown as Attribute[]

const statuses = [{ id: 's1', name: 'Under repair' }]

const state: AssetListState = {
  fields: [],
  q: 'dell',
  typeId: 'L',
  statusId: 's1',
  statusKind: 'in_use',
  includeRetired: true,
  filters: [
    { key: 'ram_gb', op: 'gte', value: '16' },
    { key: 'os', op: 'in', value: 'w,m' },
    { key: 'has_dock', op: 'eq', value: 'true' },
  ],
  sort: 'name',
  page: 3,
}

describe('attrFilterLabel', () => {
  it('reads like the condition, with units and option labels', () => {
    expect(attrFilterLabel(state.filters[0], attrs)).toBe('RAM ≥ 16 GB')
    expect(attrFilterLabel(state.filters[1], attrs)).toBe('OS any of Windows, macOS')
    expect(attrFilterLabel(state.filters[2], attrs)).toBe('Dock = Yes')
    expect(attrFilterLabel({ key: 'gone', op: 'eq', value: 'x' }, attrs)).toBe('gone = x')
  })
})

describe('filterChips', () => {
  it('lists every active filter, attribute filters last', () => {
    expect(filterChips(state, attrs, statuses).map((c) => c.label)).toEqual([
      'Search: “dell”',
      'Status: Under repair',
      'Status kind: in use',
      'Including retired',
      'RAM ≥ 16 GB',
      'OS any of Windows, macOS',
      'Dock = Yes',
    ])
  })

  it('is empty without filters', () => {
    expect(filterChips({ q: '', includeRetired: false, filters: [], fields: [], page: 1 }, attrs, statuses)).toEqual([])
  })
})

describe('removeChip', () => {
  it('clears just that filter and goes back to page 1', () => {
    const chips = filterChips(state, attrs, statuses)
    expect(removeChip(state, chips[0])).toEqual({ q: '', page: 1 })
    expect(removeChip(state, chips[1])).toEqual({ statusId: undefined, page: 1 })
    expect(removeChip(state, chips[3])).toEqual({ includeRetired: false, page: 1 })
    expect(removeChip(state, chips[5])).toEqual({ filters: [state.filters[0], state.filters[2]], page: 1 })
  })
})

describe('v1.0.2 labels and built-in chips', () => {
  it('uses = and any of', () => {
    expect(OP_LABEL.eq).toBe('=')
    expect(OP_LABEL.in).toBe('any of')
  })
  it('labels built-in conditions', () => {
    expect(fieldFilterLabel({ key: 'purchase_date', op: 'gte', value: '2026-01-01' })).toBe('Purchase date ≥ 2026-01-01')
    expect(fieldFilterLabel({ key: 'description', op: 'contains', value: 'dock' })).toBe('Description contains dock')
  })
  it('adds a removable chip per built-in condition', () => {
    const s = parseAssetQuery({ field: ['created_at:eq:2026-10-08', 'description:contains:dock'] })
    const chips = filterChips(s, [], [])
    expect(chips.map((c) => c.label)).toEqual(['Created = 2026-10-08', 'Description contains dock'])
    expect(removeChip(s, chips[0])).toEqual({ fields: [{ key: 'description', op: 'contains', value: 'dock' }], page: 1 })
  })
})
