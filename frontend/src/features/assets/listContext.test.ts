import { describe, expect, it } from 'vitest'
import { type ListContext, position, stepFrom } from './listContext'

const ctx = (page: number, ids: string[], total: number): ListContext => ({
  state: { q: '', includeRetired: false, filters: [], fields: [], page },
  pageSize: 3,
  ids,
  total,
})

describe('position', () => {
  it('counts across pages', () => {
    expect(position(ctx(2, ['d', 'e', 'f'], 7), 'e')).toEqual({ index: 4, total: 7 })
    expect(position(ctx(1, ['a', 'b', 'c'], 7), 'x')).toBeNull()
  })
})

describe('stepFrom', () => {
  it('moves inside the page', () => {
    expect(stepFrom(ctx(2, ['d', 'e', 'f'], 7), 'e', 1)).toEqual({ id: 'f' })
    expect(stepFrom(ctx(2, ['d', 'e', 'f'], 7), 'e', -1)).toEqual({ id: 'd' })
  })

  it('crosses to the neighbouring page', () => {
    expect(stepFrom(ctx(2, ['d', 'e', 'f'], 7), 'f', 1)).toEqual({ page: 3, pick: 'first' })
    expect(stepFrom(ctx(2, ['d', 'e', 'f'], 7), 'd', -1)).toEqual({ page: 1, pick: 'last' })
  })

  it('stops at the ends and for assets outside the list', () => {
    expect(stepFrom(ctx(1, ['a', 'b', 'c'], 7), 'a', -1)).toBeNull()
    expect(stepFrom(ctx(3, ['g'], 7), 'g', 1)).toBeNull()
    expect(stepFrom(ctx(1, ['a', 'b', 'c'], 7), 'x', 1)).toBeNull()
  })
})
