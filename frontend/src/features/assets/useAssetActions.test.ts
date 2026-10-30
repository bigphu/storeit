import { afterEach, describe, expect, it, vi } from 'vitest'
import { type Notice, onNotice } from '@/lib/notify'
import { useAssetActions } from './useAssetActions'

const api = vi.hoisted(() => ({
  fetchAsset: vi.fn(),
  restore: vi.fn(),
  retire: vi.fn(),
  record: vi.fn(),
}))
vi.mock('vue-router', () => ({ useRouter: () => ({}) }))
vi.mock('./api', () => ({
  fetchAsset: api.fetchAsset,
  useRestoreAsset: () => ({ mutateAsync: api.restore }),
  useRetireAsset: () => ({ mutateAsync: api.retire }),
  useRecordReplacement: () => ({ mutateAsync: api.record }),
  useReplaceAsset: () => ({ mutateAsync: vi.fn() }),
  useUndoReplacement: () => ({ mutateAsync: vi.fn() }),
}))

const flush = () => new Promise((r) => setTimeout(r))
let seen: Notice[] = []
let off = onNotice((n) => seen.push(n))
afterEach(() => {
  off()
  seen = []
  off = onNotice((n) => seen.push(n))
  vi.clearAllMocks()
})

const row = { id: 'a1', tag: 'LAP-0017', name: 'Old laptop', version: 3, retired_at: '2026-10-20T00:00:00Z' }

describe('restore', () => {
  it('retires again on Undo when the asset was not replaced', async () => {
    api.fetchAsset.mockResolvedValue({ ...row, retired_reason: 'broken', replaced_by: null })
    api.restore.mockResolvedValue({ ...row, version: 4 })
    api.retire.mockResolvedValue({ ...row, version: 5 })
    await useAssetActions().restore(row)
    expect(seen[0].summary).toBe('LAP-0017 restored.')
    seen[0].undo!()
    await flush()
    expect(api.retire).toHaveBeenCalledWith({ id: 'a1', reason: 'broken', version: 4 })
    expect(api.record).not.toHaveBeenCalled()
  })

  // restore trên tài sản đã bị thay là hoàn tác thay thế; Undo phải ghi lại đúng lần thay
  it('records the replacement again on Undo when the asset was replaced', async () => {
    const by = { id: 'b2', tag: 'LAP-0042', name: 'New laptop', replaced_on: '2026-10-20', reason: 'screen broken' }
    api.fetchAsset.mockResolvedValue({ ...row, retired_reason: 'Replaced by LAP-0042: screen broken', replaced_by: by })
    api.restore.mockResolvedValue({ ...row, version: 4 })
    api.record.mockResolvedValue({ ...row, version: 5 })
    await useAssetActions().restore(row)
    expect(seen[0].summary).toBe('Replacement undone. LAP-0017 restored.')
    seen[0].undo!()
    await flush()
    expect(api.record).toHaveBeenCalledWith({ id: 'a1', replacementId: 'b2', replacedOn: '2026-10-20', reason: 'screen broken', version: 4 })
    expect(api.retire).not.toHaveBeenCalled()
    expect(seen[1].summary).toBe('LAP-0017 replaced by LAP-0042 again.')
  })
})
