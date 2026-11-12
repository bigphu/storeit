// Hàm thuần cho trang import: lọc dòng theo kết quả, tóm tắt, và CSV các dòng lỗi
import type { ImportReport, ImportRow } from '@/lib/api/types'
import type { Tone } from '@/lib/tones'

export type ImportFilter = 'all' | ImportRow['result']

export const resultTone = (r: ImportRow['result']): Tone =>
  ({ create: 'success', update: 'info', skip: 'secondary', fail: 'danger' } as const)[r]

export const resultLabel = (r: ImportRow['result'], committed: boolean): string =>
  ({
    create: committed ? 'Created' : 'Will create',
    update: committed ? 'Updated' : 'Will update',
    skip: 'No change',
    fail: 'Failed',
  })[r]

export function filterRows(rows: ImportRow[], f: ImportFilter): ImportRow[] {
  return f === 'all' ? rows : rows.filter((r) => r.result === f)
}

// Chỉ có thể xác nhận khi còn ít nhất một dòng tạo hay sửa
export const importable = (r: ImportReport): number => r.counts.create + r.counts.update

export function summaryLine(r: ImportReport): string {
  const parts = [
    r.counts.create && `${r.counts.create} ${r.committed ? 'created' : 'to create'}`,
    r.counts.update && `${r.counts.update} ${r.committed ? 'updated' : 'to update'}`,
    r.counts.skip && `${r.counts.skip} unchanged`,
    r.counts.fail && `${r.counts.fail} failed`,
  ].filter(Boolean)
  return parts.length ? parts.join(' · ') : 'The file has no data rows'
}

const cell = (s: string) => (/[",\r\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s)

// CSV của các dòng lỗi để người dùng sửa lại file; có BOM để Excel đọc đúng tiếng Việt
export function problemsCsv(rows: ImportRow[]): string {
  const lines = [['Sheet', 'Row', 'Tag', 'Name', 'Problem'], ...rows.filter((r) => r.result === 'fail').map((r) => [r.sheet, String(r.row), r.tag, r.name, r.message])]
  return '﻿' + lines.map((l) => l.map(cell).join(',')).join('\r\n') + '\r\n'
}
