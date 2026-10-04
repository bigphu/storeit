import { describe, expect, it } from 'vitest'
import type { Status } from '@/lib/api/types'
import { archiveBlock, lanes, moveBy, orderAfterMove } from './lanes'

const st = (id: string, kind: Status['kind'], extra: Partial<Status> = {}): Status => ({
  id,
  name: id,
  kind,
  is_default: false,
  is_system: false,
  position: 0,
  ...extra,
})

const all = [
  st('a1', 'available'),
  st('u1', 'in_use'),
  st('a2', 'available', { archived_at: '2026-01-01T00:00:00Z' }),
  st('a3', 'available'),
  st('r1', 'retired'),
]

describe('lanes', () => {
  it('groups by kind keeping list order, archived last', () => {
    const l = lanes(all)
    expect(l.available.map((s) => s.id)).toEqual(['a1', 'a3', 'a2'])
    expect(l.in_use.map((s) => s.id)).toEqual(['u1'])
    expect(l.unavailable).toEqual([])
  })
})

describe('orderAfterMove', () => {
  it('puts lanes in kind order with the moved lane replaced, archived left out', () => {
    expect(orderAfterMove(all, 'available', ['a3', 'a1'])).toEqual(['a3', 'a1', 'u1', 'r1'])
  })
})

describe('moveBy', () => {
  it('swaps with the neighbour', () => {
    expect(moveBy(['x', 'y', 'z'], 'y', -1)).toEqual(['y', 'x', 'z'])
    expect(moveBy(['x', 'y', 'z'], 'y', 1)).toEqual(['x', 'z', 'y'])
  })

  it('returns null past either end or for an unknown id', () => {
    expect(moveBy(['x', 'y'], 'x', -1)).toBeNull()
    expect(moveBy(['x', 'y'], 'y', 1)).toBeNull()
    expect(moveBy(['x', 'y'], 'q', 1)).toBeNull()
  })
})

describe('archiveBlock', () => {
  it('names why built-in and default statuses stay', () => {
    expect(archiveBlock(st('s', 'available', { is_system: true, is_default: true }))).toMatch(/Built-in/)
    expect(archiveBlock(st('s', 'available', { is_default: true }))).toMatch(/default/)
    expect(archiveBlock(st('s', 'available'))).toBeUndefined()
  })
})
