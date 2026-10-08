import type { DataTableRowClickEvent, DataTableRowContextMenuEvent } from 'primevue/datatable'
import { describe, expect, it, vi } from 'vitest'
import { nextTick, ref } from 'vue'
import { captureScroll, isRowControl, onRowClick, useRowMenu, useActiveRow, rowActionsWidth, stepPage, useKeepScrollOnPage, useListTable } from './tableRows'

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

  it('opens the right-click menu with the items of the clicked row', () => {
    const m = useRowMenu<{ id: string }>((r) => [{ label: `Open ${r.id}` }])
    const show = vi.fn()
    m.context.value = { show } as never
    m.showContext(event({ id: 'a' }))
    expect(show).toHaveBeenCalled()
    expect(m.items.value).toEqual([{ label: 'Open a' }])
  })

  it('toggles the row menu button popup for that row', () => {
    const m = useRowMenu<{ id: string }>((r) => [{ label: `Open ${r.id}` }])
    const toggle = vi.fn()
    m.popup.value = { toggle } as never
    const click = {} as MouseEvent
    m.toggle({ id: 'b' }, click)
    expect(toggle).toHaveBeenCalledWith(click)
    expect(m.items.value).toEqual([{ label: 'Open b' }])
  })

  it('does not open for rows without items', () => {
    const m = useRowMenu<{ id: string }>(() => [])
    const show = vi.fn()
    m.context.value = { show } as never
    m.showContext(event({ id: 'a' }))
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


describe('stepPage', () => {
  it('moves within bounds', () => {
    expect(stepPage(2, 25, 100, 1)).toBe(3)
    expect(stepPage(2, 25, 100, -1)).toBe(1)
  })
  it('stays put on the first and last page', () => {
    expect(stepPage(1, 25, 100, -1)).toBeNull()
    expect(stepPage(4, 25, 100, 1)).toBeNull()
    expect(stepPage(1, 25, 0, 1)).toBeNull()
  })
})

describe('keeping the scroll position across pages', () => {
  const fake = () => {
    const table = { scrollLeft: 120, scrollTop: 0 }
    const page = { scrollTop: 640, scrollLeft: 0 }
    const root = { querySelector: () => table, closest: () => page }
    return { table, page, root }
  }
  it('captureScroll restores both positions', () => {
    const { table, page, root } = fake()
    const restore = captureScroll(root)!
    table.scrollLeft = 0
    page.scrollTop = 0
    restore()
    expect(page.scrollTop).toBe(640)
    expect(table.scrollLeft).toBe(120)
  })
  it('restores after the next page of data arrives', async () => {
    const { table, page, root } = fake()
    const data = ref(1)
    const keep = useKeepScrollOnPage(() => root, data)
    keep.beforePageChange()
    table.scrollLeft = 0 // bảng tự cuộn về đầu khi đổi trang
    page.scrollTop = 0
    data.value = 2
    await nextTick()
    await new Promise((r) => setTimeout(r))
    expect(page.scrollTop).toBe(640)
    expect(table.scrollLeft).toBe(120)
  })
  it('does nothing when the data changes without a page change', async () => {
    const { page, root } = fake()
    const data = ref(1)
    useKeepScrollOnPage(() => root, data)
    page.scrollTop = 10
    data.value = 2
    await new Promise((r) => setTimeout(r))
    expect(page.scrollTop).toBe(10)
  })
})
