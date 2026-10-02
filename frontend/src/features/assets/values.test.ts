import { describe, expect, it } from 'vitest'
import type { AttributeValue } from '@/lib/api/types'
import { formatValue, fromApiValues, toApiValues, type FormValues } from './values'

const attrs = [
  { key: 'serial', data_type: 'text' },
  { key: 'ram_gb', data_type: 'number' },
  { key: 'warranty_end', data_type: 'date' },
  { key: 'has_dock', data_type: 'boolean' },
  { key: 'os', data_type: 'select' },
] as const

const av = (key: string, data_type: AttributeValue['data_type'], value: unknown, extra: Partial<AttributeValue> = {}) =>
  ({ key, label: key, data_type, value, ...extra }) as AttributeValue

describe('fromApiValues', () => {
  it('turns API values into form values', () => {
    const form = fromApiValues(attrs, [
      av('serial', 'text', 'SN-1'),
      av('ram_gb', 'number', 15.6),
      av('warranty_end', 'date', '2027-06-30'),
      av('has_dock', 'boolean', false),
      av('os', 'select', 'opt-1'),
    ])
    expect(form.serial).toBe('SN-1')
    expect(form.ram_gb).toBe(15.6)
    expect(form.warranty_end).toEqual(new Date(2027, 5, 30))
    expect(form.has_dock).toBe('false')
    expect(form.os).toBe('opt-1')
  })

  it('gives empty values for attributes without a value', () => {
    expect(fromApiValues(attrs, [])).toEqual({
      serial: '',
      ram_gb: null,
      warranty_end: null,
      has_dock: null,
      os: null,
    })
  })
})

describe('toApiValues', () => {
  it('converts form values and omits empty ones', () => {
    const form: FormValues = {
      serial: '  SN-1 ',
      ram_gb: 16,
      warranty_end: new Date(2027, 5, 30),
      has_dock: 'true',
      os: 'opt-1',
    }
    expect(toApiValues(attrs, form)).toEqual({
      serial: 'SN-1',
      ram_gb: 16,
      warranty_end: '2027-06-30',
      has_dock: true,
      os: 'opt-1',
    })
    expect(toApiValues(attrs, { serial: '   ', ram_gb: null, warranty_end: null, has_dock: null, os: null })).toEqual({})
  })

  it('keeps zero and false', () => {
    expect(toApiValues(attrs, { serial: '', ram_gb: 0, warranty_end: null, has_dock: 'false', os: null })).toEqual({
      ram_gb: 0,
      has_dock: false,
    })
  })
})

describe('formatValue', () => {
  it('formats by data type', () => {
    expect(formatValue(av('ram_gb', 'number', 15.6, { unit: 'GB' }))).toBe('15.6 GB')
    expect(formatValue(av('ram_gb', 'number', 8))).toBe('8')
    expect(formatValue(av('has_dock', 'boolean', true))).toBe('Yes')
    expect(formatValue(av('has_dock', 'boolean', false))).toBe('No')
    expect(formatValue(av('os', 'select', 'x', { option_label: 'Windows' }))).toBe('Windows')
    expect(formatValue(av('os', 'select', 'x', { option_label: 'Legacy', option_removed: true }))).toBe(
      'Legacy (removed)',
    )
    expect(formatValue(av('serial', 'text', null))).toBe('—')
    expect(formatValue(undefined)).toBe('—')
  })
})
