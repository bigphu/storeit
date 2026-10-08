import { describe, expect, it, vi } from 'vitest'
import { forgetUndo, isUndoShortcut, latestUndo, type Notice, notify, onNotice, trackUndo } from './notify'

function capture() {
  const seen: Notice[] = []
  const off = onNotice((n) => seen.push(n))
  return { seen, off }
}

describe('notify', () => {
  it('passes options to listeners and picks the life', () => {
    const { seen, off } = capture()
    const run = vi.fn()
    notify.success('Downloaded a.xlsx.', { action: { label: 'What’s inside', run } })
    notify.success('LAP-1 retired.', { undo: () => {} })
    notify.success('Saved.')
    notify.error('Couldn’t save.', { detail: 'Name is taken.', retry: () => {} })
    off()
    expect(seen[0].action?.label).toBe('What’s inside')
    expect(seen[0].life).toBe(8000)
    expect(seen[1].undo).toBeTypeOf('function')
    expect(seen[1].life).toBe(8000)
    expect(seen[2].life).toBe(4000)
    expect(seen[3]).toMatchObject({ severity: 'error', summary: 'Couldn’t save.', detail: 'Name is taken.', life: 0 })
    expect(seen[3].retry).toBeTypeOf('function')
    expect(new Set(seen.map((n) => n.id)).size).toBe(4)
  })
})

describe('undo registry', () => {
  it('runs the newest visible undo, then the older one once the newest is gone', () => {
    const a = vi.fn()
    const b = vi.fn()
    trackUndo(1, a)
    trackUndo(2, b)
    latestUndo()?.()
    expect(b).toHaveBeenCalledOnce()
    forgetUndo(2)
    latestUndo()?.()
    expect(a).toHaveBeenCalledOnce()
    forgetUndo(1)
    expect(latestUndo()).toBeUndefined()
  })
})

describe('isUndoShortcut with a dialog open', () => {
  const ctrlZ = { key: 'z', ctrlKey: true, metaKey: false, shiftKey: false, altKey: false, target: { closest: () => null } } as unknown as KeyboardEvent
  it('leaves Ctrl/⌘ Z alone while a modal dialog is open (the undo would act behind it)', () => {
    expect(isUndoShortcut(ctrlZ, true)).toBe(false)
    expect(isUndoShortcut(ctrlZ, false)).toBe(true)
  })
})

describe('isUndoShortcut', () => {
  const key = (o: Partial<{ key: string; ctrlKey: boolean; metaKey: boolean; shiftKey: boolean; altKey: boolean }>, target: unknown = null) =>
    ({ key: 'z', ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, ...o, target }) as unknown as KeyboardEvent
  const field = { closest: () => ({}) }
  const plain = { closest: () => null }

  it('accepts Ctrl+Z and ⌘Z outside fields', () => {
    expect(isUndoShortcut(key({ ctrlKey: true }, plain))).toBe(true)
    expect(isUndoShortcut(key({ metaKey: true, key: 'Z' }, plain))).toBe(true)
    expect(isUndoShortcut(key({ ctrlKey: true }))).toBe(true)
  })
  it('ignores typing targets, redo and other keys', () => {
    expect(isUndoShortcut(key({ ctrlKey: true }, field))).toBe(false)
    expect(isUndoShortcut(key({ ctrlKey: true, shiftKey: true }, plain))).toBe(false)
    expect(isUndoShortcut(key({ ctrlKey: true, altKey: true }, plain))).toBe(false)
    expect(isUndoShortcut(key({ ctrlKey: true, key: 'y' }, plain))).toBe(false)
    expect(isUndoShortcut(key({}, plain))).toBe(false)
  })
})
