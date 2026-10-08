import { beforeEach, describe, expect, it, vi } from 'vitest'

const post = vi.fn()
vi.mock('@/lib/api/client', () => ({ inventoryApi: { POST: (...a: unknown[]) => post(...a) } }))
vi.mock('@/lib/download', () => ({ fileNameFrom: (_h: string | null, f: string) => f, saveBlob: vi.fn() }))

import { exportAssets, previewListQuery } from './api'

describe('exportAssets', () => {
  beforeEach(() => post.mockReset())

  it('sends the browser time zone with every export', async () => {
    post.mockResolvedValue({ data: new Blob(), response: new Response(null, { status: 200 }) })
    await exportAssets({ mode: 'data' }, 'x.xlsx')
    const body = post.mock.calls[0][1].body
    expect(body.mode).toBe('data')
    expect(body.tz).toBe(Intl.DateTimeFormat().resolvedOptions().timeZone)
    expect(body.tz).toBeTruthy()
  })
})

describe('previewListQuery', () => {
  it('sends built-in field filters with the browser time zone, like the export', () => {
    const q = previewListQuery({ q: 'x', field: ['created_at:eq:2026-10-08'], attr: ['ram_gb:gte:16'] }, 't1')
    expect(q.field).toEqual(['created_at:eq:2026-10-08'])
    expect(q.tz).toBe(Intl.DateTimeFormat().resolvedOptions().timeZone)
    expect(q.type_id).toBe('t1')
    expect(q.attr).toBeUndefined() // không có type_id trong bộ lọc: thuộc tính không áp
    expect(q.page_size).toBe(20)
  })
  it('sends no tz without built-in filters', () => {
    expect(previewListQuery({}, 't1').tz).toBeUndefined()
  })
})
