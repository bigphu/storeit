import { describe, expect, it } from 'vitest'
import { retiredItems, statusGroups, summarizeBulk } from './bulk'

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

describe('bulk undo', () => {
  const rows = [
    { id: 'a', tag: 'A', version: 3, status_id: 's1' },
    { id: 'b', tag: 'B', version: 1, status_id: 's2' },
    { id: 'c', tag: 'C', version: 7, status_id: 's1' },
    { id: 'd', tag: 'D', version: 2, status_id: 'target' },
  ]
  const result = { succeeded: ['a', 'b', 'd'], failed: [{ id: 'c', problem: { type: 't', title: 'Changed', status: 409 } }] }

  it('restores only the assets that were retired, at their new version', () => {
    expect(retiredItems(result, rows)).toEqual([
      { id: 'a', version: 4 },
      { id: 'b', version: 2 },
      { id: 'd', version: 3 },
    ])
  })

  it('groups status undo by previous status and skips assets that already had the target', () => {
    expect(statusGroups(result, rows, 'target')).toEqual([
      { statusId: 's1', items: [{ id: 'a', version: 4 }] },
      { statusId: 's2', items: [{ id: 'b', version: 2 }] },
    ])
  })
})
