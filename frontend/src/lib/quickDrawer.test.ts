import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { afterSaveNext, quickEditKey, useQuickDrawer } from './quickDrawer'

describe('useQuickDrawer', () => {
  it('keeps the item while the drawer slides out, then clears it and the draft', () => {
    const item = ref<string | null>(null)
    const clear = vi.fn()
    const d = useQuickDrawer(item, clear)
    d.open('a')
    expect(item.value).toBe('a')
    expect(d.visible.value).toBe(true)
    d.visible.value = false
    // còn trượt ra: vẫn còn mục để vẽ
    expect(item.value).toBe('a')
    clear.mockClear()
    d.closed()
    expect(item.value).toBeNull()
    expect(clear).toHaveBeenCalledOnce()
  })
  it('does not clear when reopened before the slide-out finished', () => {
    const item = ref<string | null>(null)
    const clear = vi.fn()
    const d = useQuickDrawer(item, clear)
    d.open('a')
    d.visible.value = false
    d.open('b')
    clear.mockClear()
    d.closed()
    expect(item.value).toBe('b')
    expect(clear).not.toHaveBeenCalled()
  })
})

describe('quickEditKey', () => {
  const key = (k: string, mods: Partial<{ ctrlKey: boolean; metaKey: boolean; altKey: boolean }> = {}) => ({
    key: k, ctrlKey: false, metaKey: false, altKey: false, ...mods,
  })
  const s = { dirty: false, busy: false, canPrev: true, canNext: true, typing: false, inDialog: false }
  it('J and K move between rows; arrows no longer do', () => {
    expect(quickEditKey(key('j'), s)).toBe('prev')
    expect(quickEditKey(key('k'), s)).toBe('next')
    expect(quickEditKey(key('ArrowDown'), s)).toBeNull()
    expect(quickEditKey(key('ArrowUp'), s)).toBeNull()
  })
  it('ignores J/K while typing or at the ends', () => {
    expect(quickEditKey(key('k'), { ...s, typing: true })).toBeNull()
    expect(quickEditKey(key('k'), { ...s, canNext: false })).toBeNull()
    expect(quickEditKey(key('j'), { ...s, canPrev: false })).toBeNull()
  })
  it('Ctrl S saves only with changes', () => {
    expect(quickEditKey(key('s', { ctrlKey: true }), { ...s, dirty: true })).toBe('save')
    expect(quickEditKey(key('s', { ctrlKey: true }), s)).toBeNull()
  })
  it('Ctrl Enter saves then moves on, or just moves on when nothing changed, even while typing', () => {
    expect(quickEditKey(key('Enter', { ctrlKey: true }), { ...s, dirty: true, typing: true })).toBe('saveNext')
    expect(quickEditKey(key('Enter', { metaKey: true }), s)).toBe('next')
    expect(quickEditKey(key('Enter', { ctrlKey: true }), { ...s, busy: true, dirty: true })).toBeNull()
  })
  it('Esc closes unless it came from a confirmation dialog', () => {
    expect(quickEditKey(key('Escape'), s)).toBe('close')
    expect(quickEditKey(key('Escape'), { ...s, inDialog: true })).toBeNull()
  })
})

describe('afterSaveNext', () => {
  it('waits while saving, moves on after a clean save, stays after a failed one', () => {
    expect(afterSaveNext(true, { busy: true, dirty: true, canNext: true })).toBe('wait')
    expect(afterSaveNext(false, { busy: false, dirty: true, canNext: true })).toBe('wait')
    expect(afterSaveNext(true, { busy: false, dirty: false, canNext: true })).toBe('next')
    expect(afterSaveNext(true, { busy: false, dirty: true, canNext: true })).toBe('stay')
    expect(afterSaveNext(true, { busy: false, dirty: false, canNext: false })).toBe('stay')
  })
})
