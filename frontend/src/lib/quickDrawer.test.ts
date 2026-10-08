import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { useQuickDrawer } from './quickDrawer'

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
