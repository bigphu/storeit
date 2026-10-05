// Trang status chia theo kind thành bốn "làn"; thứ tự trong làn là thứ tự chung của API
// (position). Đổi thứ tự một làn thì gửi lại thứ tự của mọi status đang dùng.
import type { Status, StatusKind } from '@/lib/api/types'

export const KIND_ORDER: StatusKind[] = ['available', 'in_use', 'unavailable', 'retired']

export const KIND_INFO: Record<StatusKind, { label: string; text: string }> = {
  available: { label: 'Available', text: 'Ready to hand out. New and restored assets get the default.' },
  in_use: { label: 'In use', text: 'Held by someone or deployed somewhere.' },
  unavailable: { label: 'Unavailable', text: "Can't be used for now: repair, lost, waiting for parts." },
  retired: { label: 'Retired', text: 'Out of service. Only Retire sets these, using the default.' },
}

// lanes: status theo kind, giữ thứ tự của danh sách (đang dùng trước, đã archive sau)
export function lanes(statuses: Status[]): Record<StatusKind, Status[]> {
  const out = { available: [], in_use: [], unavailable: [], retired: [] } as Record<StatusKind, Status[]>
  for (const s of statuses) out[s.kind].push(s)
  for (const k of KIND_ORDER) out[k].sort((a, b) => Number(!!a.archived_at) - Number(!!b.archived_at))
  return out
}

// orderAfterMove: id mọi status đang dùng theo thứ tự mới, sau khi một làn có thứ tự
// laneIds (chỉ status đang dùng của làn đó)
export function orderAfterMove(statuses: Status[], kind: StatusKind, laneIds: string[]): string[] {
  const active = statuses.filter((s) => !s.archived_at)
  return KIND_ORDER.flatMap((k) => (k === kind ? laneIds : active.filter((s) => s.kind === k).map((s) => s.id)))
}

// moveBy: thứ tự làn sau khi dời một status lên (-1) hay xuống (+1); ra ngoài làn thì null
export function moveBy(laneIds: string[], id: string, delta: number): string[] | null {
  const i = laneIds.indexOf(id)
  const j = i + delta
  if (i < 0 || j < 0 || j >= laneIds.length) return null
  const out = [...laneIds]
  ;[out[i], out[j]] = [out[j], out[i]]
  return out
}

// archiveBlock: lý do không archive được (để tắt nút và hiện tooltip), không có thì undefined
export function archiveBlock(s: Status): string | undefined {
  if (s.is_system) return "Built-in statuses can't be archived"
  if (s.is_default) return 'Make another status the default first'
  return undefined
}
