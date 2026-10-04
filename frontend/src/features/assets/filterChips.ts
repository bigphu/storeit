// Bộ lọc đang áp dụng dưới dạng chip (bấm ✕ để bỏ từng cái) và cách đọc một điều kiện
// thuộc tính thành chữ ("RAM ≥ 16 GB"), dùng cho chip và tiêu đề tab.
import type { Attribute } from '@/lib/api/types'
import type { AssetListState, AttrFilterRow } from './listQuery'

export const OP_LABEL: Record<string, string> = {
  eq: 'is',
  contains: 'contains',
  gt: '>',
  gte: '≥',
  lt: '<',
  lte: '≤',
  in: 'is any of',
}

type AttrLike = Pick<Attribute, 'key' | 'label' | 'data_type' | 'unit'> & { options: { id: string; label: string }[] }

export function attrFilterLabel(f: AttrFilterRow, attrs: readonly AttrLike[]): string {
  const a = attrs.find((x) => x.key === f.key)
  const op = OP_LABEL[f.op] ?? f.op
  if (!a) return `${f.key} ${op} ${f.value}`
  let value = f.value
  if (a.data_type === 'select') value = f.value.split(',').map((id) => a.options.find((o) => o.id === id)?.label ?? id).join(', ')
  else if (a.data_type === 'boolean') value = f.value === 'true' ? 'Yes' : 'No'
  else if (a.unit) value = `${f.value} ${a.unit}`
  return `${a.label} ${op} ${value}`
}

export type FilterChip =
  | { kind: 'search' | 'status' | 'statusKind' | 'retired'; label: string }
  | { kind: 'attr'; label: string; index: number }

export function filterChips(
  s: AssetListState,
  attrs: readonly AttrLike[],
  statuses: readonly { id: string; name: string }[],
): FilterChip[] {
  const chips: FilterChip[] = []
  if (s.q) chips.push({ kind: 'search', label: `Search: “${s.q}”` })
  if (s.statusId) chips.push({ kind: 'status', label: `Status: ${statuses.find((x) => x.id === s.statusId)?.name ?? '…'}` })
  if (s.statusKind) chips.push({ kind: 'statusKind', label: `Status kind: ${s.statusKind.replace('_', ' ')}` })
  if (s.includeRetired) chips.push({ kind: 'retired', label: 'Including retired' })
  s.filters.forEach((f, index) => chips.push({ kind: 'attr', label: attrFilterLabel(f, attrs), index }))
  return chips
}

// removeChip: thay đổi state để bỏ một chip (luôn về trang 1)
export function removeChip(s: AssetListState, chip: FilterChip): Partial<AssetListState> {
  switch (chip.kind) {
    case 'search':
      return { q: '', page: 1 }
    case 'status':
      return { statusId: undefined, page: 1 }
    case 'statusKind':
      return { statusKind: undefined, page: 1 }
    case 'retired':
      return { includeRetired: false, page: 1 }
    case 'attr':
      return { filters: s.filters.filter((_, i) => i !== chip.index), page: 1 }
  }
}
