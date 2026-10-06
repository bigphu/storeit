import { beforeEach, describe, expect, it, vi } from 'vitest'

const post = vi.fn()
vi.mock('@/lib/api/client', () => ({ inventoryApi: { POST: (...a: unknown[]) => post(...a) } }))
vi.mock('@/lib/download', () => ({ fileNameFrom: (_h: string | null, f: string) => f, saveBlob: vi.fn() }))

import { exportAssets } from './api'

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
