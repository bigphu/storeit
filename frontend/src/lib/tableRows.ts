// Hành vi chung của dòng bảng (như bảng tài sản): bấm vào dòng để mở, menu chuột phải.
// CSS đi kèm ở base.css: .clickable-row (con trỏ) và .row-actions (nút hiện khi rê chuột).
import type ContextMenu from 'primevue/contextmenu'
import type { DataTableRowClickEvent, DataTableRowContextMenuEvent } from 'primevue/datatable'
import type { MenuItem } from 'primevue/menuitem'
import { computed, type Ref, ref, shallowRef } from 'vue'

// Bấm trúng liên kết, nút, ô nhập, checkbox hay tay kéo thì để phần tử đó tự xử lý
const CONTROLS = 'a, button, input, label, textarea, .p-checkbox, .p-datatable-reorderable-row-handle'

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

export type ContextMenuRef = Ref<InstanceType<typeof ContextMenu> | undefined>

// Menu chuột phải: `menu` là ref của <ContextMenu>; gắn `items` vào :model, `show` vào
// @row-contextmenu, `clear` vào @hide. Dòng không có mục nào thì không mở menu.
export function useRowMenu<T>(menu: ContextMenuRef, build: (row: T) => MenuItem[]) {
  const row = shallowRef<T | null>(null)
  const items = computed(() => (row.value ? build(row.value) : []))
  return {
    items,
    show(e: DataTableRowContextMenuEvent) {
      if (!build(e.data as T).length) return
      row.value = e.data as T
      menu.value?.show(e.originalEvent)
    },
    clear() {
      row.value = null
    },
  }
}

// Hàng "đang dùng" của bảng dài: hàng dưới chuột hay hàng đang có focus. Bảng chỉ dựng nút
// hành động cho hàng này: mỗi nút PrimeVue tốn công dựng, 50 dòng × 3 nút làm chậm lần mở
// trang. Màn cảm ứng không rê chuột được nên dựng cho mọi hàng. Gắn onOver/onLeave/onFocusIn/
// onFocusOut lên DataTable; isActive nhận index trong slot #body (trùng data-p-index của <tr>)
type RowTarget = { closest?: (s: string) => unknown } | null

function rowIndex(target: EventTarget | null): number | null {
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
