// Kết quả thao tác hàng loạt thành câu thông báo và danh sách tài sản không làm được
import type { components } from '@/lib/api/inventory'

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
