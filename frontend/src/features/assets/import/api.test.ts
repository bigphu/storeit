import { beforeEach, describe, expect, it, vi } from 'vitest'

const post = vi.fn()
vi.mock('@/lib/api/client', () => ({ inventoryApi: { POST: (...a: unknown[]) => post(...a) } }))

import { postImport } from './api'

describe('postImport', () => {
  beforeEach(() => post.mockReset())

  it('sends the raw file with the mode and the file name', async () => {
    post.mockResolvedValue({ data: { committed: false }, response: new Response(null, { status: 200 }) })
    const file = new File(['x'], 'my assets.xlsx')
    await postImport(file, 'preview')
    const [path, opts] = post.mock.calls[0]
    expect(path).toBe('/assets/import')
    expect(opts.params.query).toEqual({ mode: 'preview', file_name: 'my assets.xlsx' })
    expect(opts.headers['Content-Type']).toBe('application/octet-stream')
    expect(opts.bodySerializer(file)).toBe(file)
  })
})
