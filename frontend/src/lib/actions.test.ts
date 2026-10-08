import { afterEach, describe, expect, it, vi } from 'vitest'
import { announce, runAction } from './actions'
import { ApiError } from './errors'
import { type Notice, onNotice } from './notify'

const flush = () => new Promise((r) => setTimeout(r))
let seen: Notice[] = []
let off = onNotice((n) => seen.push(n))
afterEach(() => {
  off()
  seen = []
  off = onNotice((n) => seen.push(n))
})

describe('runAction', () => {
  it('runs, reports done with Undo, and Undo reverses with the run result', async () => {
    const undo = vi.fn().mockResolvedValue(undefined)
    const ok = await runAction({ run: async () => ({ version: 4 }), done: 'Laptop archived.', undo, undone: 'Laptop restored.' })
    expect(ok).toBe(true)
    expect(seen[0]).toMatchObject({ severity: 'success', summary: 'Laptop archived.' })
    seen[0].undo!()
    await flush()
    expect(undo).toHaveBeenCalledWith({ version: 4 })
    expect(seen[1]).toMatchObject({ severity: 'success', summary: 'Laptop restored.' })
  })

  it('has no Undo button without undo, and done may read the result', async () => {
    await runAction({ run: async () => 3, done: (n) => `Ended ${n} sessions.` })
    expect(seen[0].summary).toBe('Ended 3 sessions.')
    expect(seen[0].undo).toBeUndefined()
  })

  it('says what is true now when Undo fails', async () => {
    const err = new ApiError({ type: '/errors/status-changed', title: 'Changed', status: 409, detail: 'Someone else changed it.' })
    await runAction({
      run: async () => null,
      done: 'Laptop archived.',
      undo: () => Promise.reject(err),
      undoFailed: "Couldn't restore Laptop. It is still archived.",
    })
    seen[0].undo!()
    await flush()
    expect(seen[1]).toMatchObject({ severity: 'error', summary: "Couldn't restore Laptop. It is still archived.", detail: 'Someone else changed it.' })
  })

  it('reports a failed run with Retry and returns false', async () => {
    const run = vi.fn().mockRejectedValueOnce(new Error('boom')).mockResolvedValueOnce(1)
    const ok = await runAction({ run, done: 'Saved.', failed: "Couldn't save." })
    expect(ok).toBe(false)
    expect(seen).toHaveLength(1)
    expect(seen[0]).toMatchObject({ severity: 'error', summary: "Couldn't save.", detail: 'boom' })
    seen[0].retry!()
    await flush()
    expect(run).toHaveBeenCalledTimes(2)
    expect(seen[1]).toMatchObject({ severity: 'success', summary: 'Saved.' })
  })
})

describe('runAction after', () => {
  it('runs the follow-up after success, also when it succeeds on Retry', async () => {
    const after = vi.fn()
    const run = vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce('r')
    expect(await runAction({ run, done: 'Deleted.', after })).toBe(false)
    expect(after).not.toHaveBeenCalled()
    seen[0].retry!()
    await flush()
    expect(after).toHaveBeenCalledWith('r')
  })
})

describe('announce', () => {
  it('runs the undo once even if triggered twice', async () => {
    const undo = vi.fn().mockResolvedValue(undefined)
    announce('r', { done: 'Saved.', undo })
    seen[0].undo!()
    seen[0].undo!()
    await flush()
    expect(undo).toHaveBeenCalledOnce()
  })
})
