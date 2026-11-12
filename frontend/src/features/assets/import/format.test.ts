import { describe, expect, it } from 'vitest'
import type { ImportReport, ImportRow } from '@/lib/api/types'
import { filterRows, importable, problemsCsv, resultLabel, summaryLine } from './format'

const row = (o: Partial<ImportRow>): ImportRow => ({ sheet: 'LAPTOP', row: 2, tag: 'A-1', name: 'n', result: 'create', message: '', ...o })
const report = (counts: Partial<ImportReport['counts']>, committed = false): ImportReport => ({
  file_name: 'a.xlsx',
  committed,
  sheets: [],
  counts: { create: 0, update: 0, skip: 0, fail: 0, ...counts },
  rows: [],
})

describe('import format', () => {
  it('filters rows by result', () => {
    const rows = [row({ result: 'create' }), row({ result: 'fail' }), row({ result: 'fail', row: 4 })]
    expect(filterRows(rows, 'all')).toHaveLength(3)
    expect(filterRows(rows, 'fail').map((r) => r.row)).toEqual([2, 4])
  })

  it('words the summary for a preview and for a finished import', () => {
    expect(summaryLine(report({ create: 2, update: 1, fail: 3 }))).toBe('2 to create · 1 to update · 3 failed')
    expect(summaryLine(report({ create: 2, skip: 1 }, true))).toBe('2 created · 1 unchanged')
    expect(summaryLine(report({}))).toBe('The file has no data rows')
  })

  it('counts what a commit would write', () => {
    expect(importable(report({ create: 1, update: 2, skip: 5, fail: 1 }))).toBe(3)
  })

  it('labels results by tense', () => {
    expect(resultLabel('create', false)).toBe('Will create')
    expect(resultLabel('create', true)).toBe('Created')
  })

  it('writes only failed rows to CSV and quotes awkward text', () => {
    const csv = problemsCsv([row({ result: 'create' }), row({ result: 'fail', row: 7, name: 'Dell, "XPS"', message: 'Status "X" not found' })])
    expect(csv).toBe('﻿Sheet,Row,Tag,Name,Problem\r\nLAPTOP,7,A-1,"Dell, ""XPS""","Status ""X"" not found"\r\n')
  })
})
