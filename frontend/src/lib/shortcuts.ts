// Mọi phím tắt của app ở một chỗ: bảng phím tắt (?) và gợi ý trên nút đọc từ đây, nên chữ
// và phím không lệch nhau. Mỗi phần tử của keys là một <kbd>.
export type ShortcutGroup = 'Anywhere' | 'Lists' | 'Asset page' | 'Quick edit' | 'Forms'

export interface Shortcut {
  keys: string[]
  action: string
  group: ShortcutGroup
}

export const GROUP_ORDER: ShortcutGroup[] = ['Anywhere', 'Lists', 'Asset page', 'Quick edit', 'Forms']

export const SHORTCUTS: readonly Shortcut[] = [
  { group: 'Anywhere', keys: ['?'], action: 'Show keyboard shortcuts' },
  { group: 'Anywhere', keys: ['Ctrl K'], action: 'Go to an asset type (keeps the section you are in)' },
  { group: 'Anywhere', keys: ['Alt 1–9'], action: 'Go to tab 1–9' },
  { group: 'Anywhere', keys: ['Ctrl Z'], action: 'Undo the change in the latest message' },
  { group: 'Anywhere', keys: ['Ctrl-click', 'Middle-click'], action: 'Open a link in a new tab' },
  { group: 'Anywhere', keys: ['Esc'], action: 'Close a dialog, menu or drawer' },
  { group: 'Lists', keys: ['J', 'K'], action: 'Previous / next page' },
  { group: 'Lists', keys: ['/'], action: 'Search the list' },
  { group: 'Lists', keys: ['N'], action: 'New item (asset, type, role, export profile, or invite an account)' },
  { group: 'Lists', keys: ['F'], action: 'Add a filter (asset list)' },
  { group: 'Lists', keys: ['X'], action: 'Export the list (data .xlsx, asset list)' },
  { group: 'Lists', keys: ['R'], action: 'Customize a report of the list (asset list)' },
  { group: 'Lists', keys: ['Shift + wheel'], action: 'Scroll a wide table sideways' },
  { group: 'Asset page', keys: ['J', 'K'], action: 'Previous / next asset' },
  { group: 'Asset page', keys: ['E'], action: 'Edit the asset' },
  { group: 'Quick edit', keys: ['J', 'K'], action: 'Previous / next row' },
  { group: 'Quick edit', keys: ['Ctrl S'], action: 'Save' },
  { group: 'Quick edit', keys: ['Ctrl Enter'], action: 'Save, then open the next row' },
  { group: 'Forms', keys: ['Ctrl S'], action: 'Save' },
]

export function shortcutGroups(): { group: ShortcutGroup; items: Shortcut[] }[] {
  return GROUP_ORDER.map((group) => ({ group, items: SHORTCUTS.filter((s) => s.group === group) }))
}
