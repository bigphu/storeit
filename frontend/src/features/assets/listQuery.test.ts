import { describe, expect, it } from 'vitest'
import {
  type AssetListState,
  changeType,
  fromTableSort,
  operatorsFor,
  parseAssetQuery,
  serializeAssetQuery,
  toApiParams,
  toTableSort,
} from './listQuery'

const base: AssetListState = { q: '', includeRetired: false, filters: [], page: 1 }

describe('URL <-> state', () => {
  it('parses and serializes every field', () => {
    const query = {
      q: 'dell',
      type_id: 'T',
      status_id: 'S',
      status_kind: 'in_use',
      include_retired: 'true',
      attr: ['ram_gb:gte:16', 'note:contains:a:b'],
      sort: '-attributes.ram_gb',
      page: '3',
    }
    const state = parseAssetQuery(query)
    expect(state).toEqual({
      q: 'dell',
      typeId: 'T',
      statusId: 'S',
      statusKind: 'in_use',
      includeRetired: true,
      // ':' trong giá trị là chữ
      filters: [
        { key: 'ram_gb', op: 'gte', value: '16' },
        { key: 'note', op: 'contains', value: 'a:b' },
      ],
      sort: '-attributes.ram_gb',
      page: 3,
    })
    expect(serializeAssetQuery(state)).toEqual(query)
  })

  it('drops defaults and junk', () => {
    expect(parseAssetQuery({ attr: 'bad', page: '-2', status_kind: 'nope', include_retired: 'x' })).toEqual(base)
    expect(serializeAssetQuery(base)).toEqual({})
  })

  it('reads a single attr param', () => {
    expect(parseAssetQuery({ type_id: 'T', attr: 'has_dock:eq:true' }).filters).toEqual([
      { key: 'has_dock', op: 'eq', value: 'true' },
    ])
  })
})

describe('toApiParams', () => {
  it('maps state to listAssets query, skipping empty filter rows', () => {
    const state: AssetListState = {
      ...base,
      q: 'x',
      typeId: 'T',
      filters: [
        { key: 'ram_gb', op: 'gte', value: '16' },
        { key: 'note', op: 'eq', value: '' },
      ],
      sort: 'name',
      page: 2,
    }
    expect(toApiParams(state, 25)).toEqual({
      q: 'x',
      type_id: 'T',
      include_retired: false,
      attr: ['ram_gb:gte:16'],
      sort: 'name',
      page: 2,
      page_size: 25,
    })
  })

  it('never sends attribute filters or sort without a type', () => {
    const state: AssetListState = {
      ...base,
      filters: [{ key: 'ram_gb', op: 'gte', value: '16' }],
      sort: 'attributes.ram_gb',
    }
    const p = toApiParams(state, 50)
    expect(p.attr).toBeUndefined()
    expect(p.sort).toBeUndefined()
  })
})

describe('changeType', () => {
  it('clears attribute filters and an attribute sort, keeps a column sort', () => {
    const s: AssetListState = {
      ...base,
      typeId: 'A',
      filters: [{ key: 'ram_gb', op: 'gte', value: '16' }],
      sort: '-attributes.ram_gb',
      page: 4,
    }
    expect(changeType(s, 'B')).toEqual({ ...base, typeId: 'B' })
    expect(changeType({ ...s, sort: '-name' }, undefined)).toEqual({ ...base, sort: '-name' })
  })
})

describe('sorting', () => {
  it('converts table sort events to the API sort', () => {
    expect(fromTableSort('name', 1)).toBe('name')
    expect(fromTableSort('attributes.ram_gb', -1)).toBe('-attributes.ram_gb')
    expect(fromTableSort('name', 0)).toBeUndefined()
    expect(fromTableSort(undefined, 1)).toBeUndefined()
  })

  it('converts to the table sort state', () => {
    expect(toTableSort('-attributes.ram_gb')).toEqual({ field: 'attributes.ram_gb', order: -1 })
    expect(toTableSort('tag')).toEqual({ field: 'tag', order: 1 })
    expect(toTableSort(undefined)).toEqual({ field: undefined, order: 0 })
  })
})

describe('operatorsFor', () => {
  it('matches the backend per data type', () => {
    expect(operatorsFor('text')).toEqual(['eq', 'contains'])
    expect(operatorsFor('number')).toEqual(['eq', 'gt', 'gte', 'lt', 'lte'])
    expect(operatorsFor('date')).toEqual(['eq', 'gt', 'gte', 'lt', 'lte'])
    expect(operatorsFor('boolean')).toEqual(['eq'])
    expect(operatorsFor('select')).toEqual(['eq', 'in'])
  })
})
