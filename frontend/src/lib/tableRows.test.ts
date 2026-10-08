import type { DataTableRowClickEvent, DataTableRowContextMenuEvent } from 'primevue/datatable'
import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { isRowControl, onRowClick, useRowMenu, useActiveRow } from './tableRows'

// phần tử giả: closest trả về chính nó khi selector chứa tên "thẻ" của nó
const el = (tag: string | null) => ({ closest: (s: string) => (tag && s.split(', ').includes(tag) ? {} : null) })

describe('isRowControl', () => {
  it('is true inside links, buttons and drag handles', () => {
    expect(isRowControl(el('a') as unknown as EventTarget)).toBe(true)
    expect(isRowControl(el('button') as unknown as EventTarget)).toBe(true)
    expect(isRowControl(el('.p-datatable-reorderable-row-handle') as unknown as EventTarget)).toBe(true)
  })

  it('is false on plain cells and without a target', () => {
    expect(isRowControl(el(null) as unknown as EventTarget)).toBe(false)
    expect(isRowControl(null)).toBe(false)
  })
})

describe('onRowClick', () => {
  const click = (tag: string | null) =>
    ({ originalEvent: { target: el(tag) }, data: { id: 'r1' }, index: 0 }) as unknown as DataTableRowClickEvent

  it('opens the row with the mouse event', () => {
    const open = vi.fn()
    const e = click(null)
    onRowClick(open)(e)
    expect(open).toHaveBeenCalledWith({ id: 'r1' }, e.originalEvent)
  })

  it('leaves clicks on controls alone', () => {
    const open = vi.fn()
    onRowClick(open)(click('button'))
    expect(open).not.toHaveBeenCalled()
  })
})

describe('useRowMenu', () => {
  const event = (data: object) => ({ originalEvent: {}, data, index: 0 }) as unknown as DataTableRowContextMenuEvent

  it('builds items for the clicked row and clears on hide', () => {
    const show = vi.fn()
    const m = useRowMenu<{ id: string }>(ref({ show } as never), (r) => [{ label: `Open ${r.id}` }])
    m.show(event({ id: 'a' }))
    expect(show).toHaveBeenCalled()
    expect(m.items.value).toEqual([{ label: 'Open a' }])
    m.clear()
    expect(m.items.value).toEqual([])
  })

  it('does not open for rows without items', () => {
    const show = vi.fn()
    const m = useRowMenu<{ id: string }>(ref({ show } as never), () => [])
    m.show(event({ id: 'a' }))
    expect(show).not.toHaveBeenCalled()
  })
})

describe('useActiveRow', () => {
  // phần tử giả: closest('tbody tr[data-p-index]') trả về hàng có dataset.pIndex
  const inRow = (i: number) => ({ closest: (s: string) => (s === 'tbody tr[data-p-index]' ? { dataset: { pIndex: String(i) } } : null) })
  const outside = { closest: () => null }
  const ev = (target: unknown, relatedTarget: unknown = null) => ({ target, relatedTarget }) as unknown as FocusEvent

  it('follows the hovered row and forgets it when the pointer leaves the table', () => {
    const r = useActiveRow(false)
    expect(r.isActive(3)).toBe(false)
    r.onOver(ev(inRow(3)))
    expect(r.isActive(3)).toBe(true)
    r.onOver(ev(inRow(5)))
    expect(r.isActive(3)).toBe(false)
    expect(r.isActive(5)).toBe(true)
    r.onLeave()
    expect(r.isActive(5)).toBe(false)
  })
  it('keeps the focused row active even when the pointer is elsewhere', () => {
    const r = useActiveRow(false)
    r.onFocusIn(ev(inRow(2)))
    r.onOver(ev(inRow(7)))
    expect(r.isActive(2)).toBe(true)
    expect(r.isActive(7)).toBe(true)
    r.onLeave()
    expect(r.isActive(2)).toBe(true)
    r.onFocusOut(ev(inRow(2), outside))
    expect(r.isActive(2)).toBe(false)
  })
  it('treats every row as active on touch screens', () => {
    const r = useActiveRow(true)
    expect(r.isActive(0)).toBe(true)
    expect(r.isActive(42)).toBe(true)
  })
})
