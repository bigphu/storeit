import { describe, expect, it } from 'vitest'
import type { Attribute } from '@/lib/api/types'
import { attrFilterLabel, filterChips, removeChip } from './filterChips'
import type { AssetListState } from './listQuery'

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
    expect(attrFilterLabel(state.filters[1], attrs)).toBe('OS is any of Windows, macOS')
    expect(attrFilterLabel(state.filters[2], attrs)).toBe('Dock is Yes')
    expect(attrFilterLabel({ key: 'gone', op: 'eq', value: 'x' }, attrs)).toBe('gone is x')
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
      'OS is any of Windows, macOS',
      'Dock is Yes',
    ])
  })

  it('is empty without filters', () => {
    expect(filterChips({ q: '', includeRetired: false, filters: [], page: 1 }, attrs, statuses)).toEqual([])
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
