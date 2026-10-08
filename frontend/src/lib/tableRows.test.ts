import type { DataTableRowClickEvent, DataTableRowContextMenuEvent } from 'primevue/datatable'
import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { isRowControl, onRowClick, useRowMenu, useActiveRow, rowActionsWidth, useListTable } from './tableRows'

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

describe('rowActionsWidth', () => {
  it('reserves room for every button the column can show (2rem each, 0.25rem gaps)', () => {
    expect(rowActionsWidth(1)).toBe('2rem')
    expect(rowActionsWidth(3)).toBe('6.5rem')
  })
})

describe('useListTable', () => {
  const inRow = (i: number) => ({ closest: (s: string) => (s === 'tbody tr[data-p-index]' ? { dataset: { pIndex: String(i) } } : null) })
  const over = (i: number) => ({ target: inRow(i) }) as unknown as MouseEvent

  it('wires clickable rows, the context menu and the active row', () => {
    const open = vi.fn()
    const showMenu = vi.fn()
    const t = useListTable<{ id: string }>({ open, showMenu, touch: false })
    expect(t.bind.rowHover).toBe(true)
    expect(t.bind.rowClass?.()).toBe('clickable-row')
    t.bind.onRowClick?.({ data: { id: 'a' }, originalEvent: { target: null } } as unknown as DataTableRowClickEvent)
    expect(open).toHaveBeenCalledWith({ id: 'a' }, { target: null })
    expect(t.bind.onRowContextmenu).toBe(showMenu)
    t.bind.onMouseover(over(2))
    expect(t.active.isActive(2)).toBe(true)
  })
  it('reports the hovered row once per row and null when the pointer leaves', () => {
    const onHover = vi.fn()
    const t = useListTable({ onHover, touch: false })
    t.bind.onMouseover(over(1))
    t.bind.onMouseover(over(1))
    t.bind.onMouseover(over(4))
    t.bind.onMouseleave()
    expect(onHover.mock.calls).toEqual([[1], [4], [null]])
  })
  it('has no click or menu wiring for read-only tables', () => {
    const t = useListTable({ touch: false })
    expect(t.bind.onRowClick).toBeUndefined()
    expect(t.bind.rowClass).toBeUndefined()
    expect(t.bind.onRowContextmenu).toBeUndefined()
  })
})

describe('useListTable clickable', () => {
  it('marks and opens only the rows that can be opened', () => {
    const open = vi.fn()
    const t = useListTable<{ removed: boolean }>({ open, clickable: (r) => !r.removed, touch: false })
    expect(t.bind.rowClass?.({ removed: false })).toBe('clickable-row')
    expect(t.bind.rowClass?.({ removed: true })).toBeUndefined()
    t.bind.onRowClick?.({ data: { removed: true }, originalEvent: { target: null } } as unknown as DataTableRowClickEvent)
    expect(open).not.toHaveBeenCalled()
  })
})
