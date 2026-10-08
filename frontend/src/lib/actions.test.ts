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

describe('runAction Retry', () => {
  it('retries once even if Retry is clicked twice (the toast is still fading out)', async () => {
    const run = vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValue(1)
    await runAction({ run, done: 'Sent.' })
    seen[0].retry!()
    seen[0].retry!()
    await flush()
    expect(run).toHaveBeenCalledTimes(2)
  })
  it('offers no Retry for client errors that would fail the same way again', async () => {
    const taken = new ApiError({ type: '/errors/name-taken', title: 'Name taken', status: 409 })
    await runAction({ run: () => Promise.reject(taken), done: 'Saved.' })
    expect(seen[0].retry).toBeUndefined()
  })
  it('offers Retry for server errors and rate limits', async () => {
    const down = new ApiError({ type: '/errors/internal', title: 'Server error', status: 503 })
    const busy = new ApiError({ type: '/errors/rate-limited', title: 'Too many requests', status: 429 })
    await runAction({ run: () => Promise.reject(down), done: 'Saved.' })
    await runAction({ run: () => Promise.reject(busy), done: 'Saved.' })
    expect(seen[0].retry).toBeTypeOf('function')
    expect(seen[1].retry).toBeTypeOf('function')
  })
})

// Undo đặt lại cả danh sách (quyền, role): đã có thay đổi mới hơn trên cùng thứ thì không đè
describe('undoKey', () => {
  it('refuses to undo an older change once a newer one on the same thing exists', async () => {
    const undoOld = vi.fn().mockResolvedValue(undefined)
    const undoNew = vi.fn().mockResolvedValue(undefined)
    await runAction({ run: async () => 1, done: 'Roles saved.', undo: undoOld, undoKey: 'account-roles:a1' })
    await runAction({ run: async () => 2, done: 'Roles saved.', undo: undoNew, undoKey: 'account-roles:a1' })
    seen[0].undo!()
    await flush()
    expect(undoOld).not.toHaveBeenCalled()
    expect(seen.at(-1)).toMatchObject({ severity: 'error' })
    seen[1].undo!()
    await flush()
    expect(undoNew).toHaveBeenCalledOnce()
  })
  it('keeps undo for changes to different things', async () => {
    const undoA = vi.fn().mockResolvedValue(undefined)
    await runAction({ run: async () => 1, done: 'Saved.', undo: undoA, undoKey: 'account-roles:a1' })
    await runAction({ run: async () => 2, done: 'Saved.', undo: async () => {}, undoKey: 'account-roles:a2' })
    seen[0].undo!()
    await flush()
    expect(undoA).toHaveBeenCalledOnce()
  })
})
