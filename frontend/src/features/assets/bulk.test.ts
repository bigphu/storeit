import { describe, expect, it } from 'vitest'
import { summarizeBulk } from './bulk'

describe('summarizeBulk', () => {
  const rows = [
    { id: 'a', tag: 'LAP-1' },
    { id: 'b', tag: 'LAP-2' },
    { id: 'c', tag: 'LAP-3' },
  ]

  it('names the failed assets with the reason a person can act on', () => {
    const s = summarizeBulk(
      {
        succeeded: ['a'],
        failed: [
          { id: 'b', problem: { type: '/errors/asset-changed', title: 'Asset changed', status: 409, detail: 'Changed by someone else.' } },
          { id: 'x', problem: { type: '/errors/asset-not-found', title: 'Asset not found', status: 404 } },
        ],
      },
      rows,
      'retired',
    )
    expect(s.message).toBe('1 asset retired. 2 could not be.')
    expect(s.failures).toEqual([
      { tag: 'LAP-2', reason: 'Changed by someone else.' },
      { tag: 'x', reason: 'Asset not found' },
    ])
  })

  it('reads well when everything worked', () => {
    expect(summarizeBulk({ succeeded: ['a', 'b'], failed: [] }, rows, 'updated').message).toBe('2 assets updated.')
  })
})
