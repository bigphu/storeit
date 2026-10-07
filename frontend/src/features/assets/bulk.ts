// Kết quả thao tác hàng loạt thành câu thông báo và danh sách tài sản không làm được
import type { components } from '@/lib/api/inventory'
import type { BulkItemRef } from './api'

export type BulkResult = components['schemas']['BulkResult']

export interface BulkSummary {
  message: string
  failures: { tag: string; reason: string }[]
}

export function summarizeBulk(r: BulkResult, rows: readonly { id: string; tag: string }[], verb: string): BulkSummary {
  const tagOf = (id: string) => rows.find((x) => x.id === id)?.tag ?? id
  const done = r.succeeded.length
  let message = `${done} ${done === 1 ? 'asset' : 'assets'} ${verb}.`
  if (r.failed.length) message += ` ${r.failed.length} could not be.`
  return {
    message,
    failures: r.failed.map((f) => ({ tag: tagOf(f.id), reason: f.problem.detail || f.problem.title })),
  }
}

// Undo của retire hàng loạt: tài sản đã retire, với version mới (mỗi lần ghi tăng 1)
export function retiredItems(r: BulkResult, rows: readonly { id: string; version: number }[]): BulkItemRef[] {
  const ok = new Set(r.succeeded)
  return rows.filter((x) => ok.has(x.id)).map((x) => ({ id: x.id, version: x.version + 1 }))
}

// Undo của đổi status hàng loạt: nhóm theo status cũ để đặt lại từng nhóm. Tài sản vốn
// đã có status đích thì server không ghi (version giữ nguyên) nên không cần đặt lại
export function statusGroups(
  r: BulkResult,
  rows: readonly { id: string; version: number; status_id: string }[],
  target: string,
): { statusId: string; items: BulkItemRef[] }[] {
  const ok = new Set(r.succeeded)
  const groups = new Map<string, BulkItemRef[]>()
  for (const x of rows) {
    if (!ok.has(x.id) || x.status_id === target) continue
    const g = groups.get(x.status_id) ?? []
    g.push({ id: x.id, version: x.version + 1 })
    groups.set(x.status_id, g)
  }
  return [...groups].map(([statusId, items]) => ({ statusId, items }))
}
