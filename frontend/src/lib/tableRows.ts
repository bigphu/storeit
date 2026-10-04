// Hành vi chung của dòng bảng (như bảng tài sản): bấm vào dòng để mở, menu chuột phải.
// CSS đi kèm ở base.css: .clickable-row (con trỏ) và .row-actions (nút hiện khi rê chuột).
import type ContextMenu from 'primevue/contextmenu'
import type { DataTableRowClickEvent, DataTableRowContextMenuEvent } from 'primevue/datatable'
import type { MenuItem } from 'primevue/menuitem'
import { computed, type Ref, shallowRef } from 'vue'

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
