// Hành vi chung của dòng bảng (như bảng tài sản): bấm vào dòng để mở, menu chuột phải.
// CSS đi kèm ở base.css: .clickable-row (con trỏ) và .row-actions (nút hiện khi rê chuột).
import type ContextMenu from 'primevue/contextmenu'
import type Menu from 'primevue/menu'
import type { DataTableRowClickEvent, DataTableRowContextMenuEvent } from 'primevue/datatable'
import type { MenuItem } from 'primevue/menuitem'
import { computed, type MaybeRefOrGetter, nextTick, ref, shallowRef, toValue, watch, type WatchSource } from 'vue'

// Bấm trúng liên kết, nút, ô nhập, checkbox (cả ô chứa checkbox chọn dòng) hay tay kéo thì
// để phần tử đó tự xử lý
const CONTROLS = 'a, button, input, label, textarea, .p-checkbox, .select-cell, .p-datatable-reorderable-row-handle'

export function isRowControl(target: EventTarget | null): boolean {
  const el = target as { closest?: (s: string) => unknown } | null
  return !!el?.closest?.(CONTROLS)
}

// Trả về handler cho @row-click; open nhận sự kiện chuột để Ctrl/⌘-click mở tab mới
export function onRowClick<T>(open: (row: T, e: MouseEvent) => void) {
  return (e: DataTableRowClickEvent) => {
    if (isRowControl(e.originalEvent.target)) return
    open(e.data as T, e.originalEvent as MouseEvent)
  }
}

// Menu của dòng: cùng một danh sách mục cho menu chuột phải (@row-contextmenu → showContext)
// và nút ☰ ở cuối dòng (RowMenuButton → toggle). RowMenus vẽ hai menu và gắn context/popup.
// Không xoá dòng đang chọn khi menu đóng: menu báo đóng sau hiệu ứng, lúc đó có thể đã mở cho
// dòng khác
export function useRowMenu<T>(build: (row: T) => MenuItem[]) {
  const row = shallowRef<T | null>(null)
  const items = computed(() => (row.value ? build(row.value) : []))
  const context = ref<InstanceType<typeof ContextMenu>>()
  const popup = ref<InstanceType<typeof Menu>>()
  return {
    items,
    context,
    popup,
    showContext(e: DataTableRowContextMenuEvent) {
      if (!build(e.data as T).length) return
      row.value = e.data as T
      context.value?.show(e.originalEvent)
    },
    toggle(r: T, e: MouseEvent) {
      row.value = r
      popup.value?.toggle(e)
    },
  }
}

export type RowMenu = ReturnType<typeof useRowMenu>

// Hàng "đang dùng" của bảng dài: hàng dưới chuột hay hàng đang có focus. Bảng chỉ dựng nút
// hành động cho hàng này: mỗi nút PrimeVue tốn công dựng, 50 dòng × 3 nút làm chậm lần mở
// trang. Màn cảm ứng không rê chuột được nên dựng cho mọi hàng. Gắn onOver/onLeave/onFocusIn/
// onFocusOut lên DataTable; isActive nhận index trong slot #body (trùng data-p-index của <tr>)
type RowTarget = { closest?: (s: string) => unknown } | null

export function rowIndex(target: EventTarget | null): number | null {
  const tr = (target as RowTarget)?.closest?.('tbody tr[data-p-index]') as { dataset: { pIndex?: string } } | null | undefined
  return tr?.dataset.pIndex == null ? null : Number(tr.dataset.pIndex)
}

export function useActiveRow(touch = typeof window !== 'undefined' && !!window.matchMedia?.('(hover: none)').matches) {
  const hovered = ref<number | null>(null)
  const focused = ref<number | null>(null)
  return {
    isActive: (i: number) => touch || hovered.value === i || focused.value === i,
    onOver(e: Event) {
      const i = rowIndex(e.target)
      if (i !== null) hovered.value = i
    },
    onLeave() {
      hovered.value = null
    },
    onFocusIn(e: Event) {
      const i = rowIndex(e.target)
      if (i !== null) focused.value = i
    },
    // focus rời hẳn hàng (không sang phần tử khác trong cùng hàng)
    onFocusOut(e: FocusEvent) {
      if (rowIndex(e.relatedTarget) !== focused.value) focused.value = null
    },
  }
}

// rowActionsWidth: bề rộng giữ sẵn cho ô nút cuối dòng có tối đa count nút (IconAction 2rem,
// cách nhau 0.25rem). Nút chỉ dựng khi rê chuột, nên ô phải giữ chỗ trước, không thì cột giãn
// ra và cả bảng xô lệch khi rê chuột
export function rowActionsWidth(count: number): string {
  return `${count * 2 + Math.max(0, count - 1) * 0.25}rem`
}

// useListTable: một chỗ gắn hành vi chung của bảng danh sách. bind gắn lên DataTable
// (v-bind="table.bind"): dòng bấm được (open; Ctrl/⌘ hay chuột giữa mở tab mới), menu chuột
// phải (showMenu của useRowMenu), hàng đang dùng cho RowActions (active.isActive(index)).
// onHover: hàng chuột vừa tới (mỗi hàng một lần), null khi chuột rời bảng. clickable: chỉ một
// số dòng mở được (vd thuộc tính đã gỡ thì không)
export function useListTable<T>(o: {
  open?: (row: T, e: MouseEvent) => void
  clickable?: (row: T) => boolean
  showMenu?: (e: DataTableRowContextMenuEvent) => void
  onHover?: (index: number | null) => void
  touch?: boolean
} = {}) {
  const active = o.touch === undefined ? useActiveRow() : useActiveRow(o.touch)
  let hovered: number | null = null
  const bind = {
    rowHover: true as const,
    rowClass: o.open ? (row?: T) => (!o.clickable || (row !== undefined && o.clickable(row)) ? 'clickable-row' : undefined) : undefined,
    onRowClick: o.open ? onRowClick<T>((row, e) => (!o.clickable || o.clickable(row)) && o.open!(row, e)) : undefined,
    onRowContextmenu: o.showMenu,
    onMouseover(e: MouseEvent) {
      active.onOver(e)
      const i = rowIndex(e.target)
      if (i === null || i === hovered) return
      hovered = i
      o.onHover?.(i)
    },
    onMouseleave() {
      active.onLeave()
      if (hovered === null) return
      hovered = null
      o.onHover?.(null)
    },
    onFocusin: active.onFocusIn,
    onFocusout: active.onFocusOut,
  }
  return { bind, active }
}

// J/K ở bảng có phân trang: số trang trước/sau, null khi đã ở trang đầu/cuối
export function stepPage(page: number, pageSize: number, total: number, dir: -1 | 1): number | null {
  const last = Math.max(1, Math.ceil(total / pageSize))
  const next = page + dir
  return next < 1 || next > last ? null : next
}

interface ScrollBox {
  scrollTop: number
  scrollLeft: number
}
export interface TableRoot {
  querySelector(selector: string): unknown
  closest(selector: string): unknown
}

// captureScroll: chụp vị trí cuộn dọc của vùng nội dung (.content) và cuộn ngang của bảng;
// hàm trả về đặt lại đúng vị trí đó
export function captureScroll(el: TableRoot | null | undefined): (() => void) | null {
  if (!el) return null
  const table = el.querySelector('.p-datatable-table-container') as ScrollBox | null
  const page = el.closest('.content') as ScrollBox | null
  const top = page?.scrollTop ?? 0
  const left = table?.scrollLeft ?? 0
  return () => {
    if (page) page.scrollTop = top
    if (table) table.scrollLeft = left
  }
}

// Đổi trang bảng mà không mất chỗ đang xem: gọi beforePageChange() ngay trước khi đổi trang,
// vị trí được trả lại khi dữ liệu của trang mới về và bảng vẽ xong
export function useKeepScrollOnPage(root: MaybeRefOrGetter<TableRoot | null | undefined>, data: WatchSource<unknown>) {
  let restore: (() => void) | null = null
  watch(data, async () => {
    const r = restore
    restore = null
    if (!r) return
    await nextTick()
    r()
  })
  return {
    beforePageChange() {
      restore = captureScroll(toValue(root))
    },
  }
}
