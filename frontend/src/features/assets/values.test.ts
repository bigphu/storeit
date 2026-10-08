import { describe, expect, it } from 'vitest'
import type { AssetDetail, AttributeValue } from '@/lib/api/types'
import { assetBodyOf, attrInputError, attrText, formatValue, formValueFrom, fromApiValues, toApiValues, type FormValues } from './values'

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

describe('assetBodyOf', () => {
  it('rebuilds the PUT body from a saved asset', () => {
    const a = {
      id: 'x', tag: 'LAP-1', name: 'Laptop', description: 'Old', version: 5,
      asset_type: { id: 't1' }, status: { id: 's1' },
      location_id: 'l1', purchase_date: '2026-01-02',
      attributes: [
        { key: 'ram', label: 'RAM', data_type: 'number', value: 16 },
        { key: 'note', label: 'Note', data_type: 'text', value: null },
        { key: 'os', label: 'OS', data_type: 'select', value: 'opt-1', option_label: 'Windows' },
      ],
    } as unknown as AssetDetail
    expect(assetBodyOf(a)).toEqual({
      name: 'Laptop', description: 'Old', asset_type_id: 't1', status_id: 's1',
      location_id: 'l1', holder_member_id: undefined, purchase_date: '2026-01-02',
      attributes: { ram: 16, os: 'opt-1' },
    })
  })
})

describe('attrText / formValueFrom', () => {
  it('gives the editable text of a list cell value', () => {
    expect(attrText({ key: 'ram', label: 'RAM', data_type: 'number', value: 16 } as AttributeValue)).toBe('16')
    expect(attrText({ key: 'os', label: 'OS', data_type: 'select', value: 'opt-1', option_label: 'Windows' } as AttributeValue)).toBe('opt-1')
    expect(attrText({ key: 'ok', label: 'OK', data_type: 'boolean', value: false } as AttributeValue)).toBe('false')
    expect(attrText({ key: 'd', label: 'D', data_type: 'date', value: '2026-03-25' } as AttributeValue)).toBe('2026-03-25')
    expect(attrText(undefined)).toBe('')
  })
  it('turns edited text back into a form value of the right type', () => {
    expect(formValueFrom('number', '32')).toBe(32)
    expect(formValueFrom('number', '')).toBeNull()
    expect(formValueFrom('date', '2026-03-25')).toEqual(new Date(2026, 2, 25))
    expect(formValueFrom('boolean', 'true')).toBe('true')
    expect(formValueFrom('boolean', '')).toBeNull()
    expect(formValueFrom('select', 'opt-2')).toBe('opt-2')
    expect(formValueFrom('text', 'Xám')).toBe('Xám')
  })
})

// Ô sửa nhanh là ô chữ: số gõ sai không được thành NaN (gửi lên thành null, xoá mất giá trị)
describe('attrInputError', () => {
  it('rejects text that is not a number for number attributes', () => {
    expect(attrInputError('number', '12kg')).toBe('Enter a number.')
    expect(attrInputError('number', ' 1e3 ')).toBeUndefined()
    expect(attrInputError('number', '-2.5')).toBeUndefined()
    expect(attrInputError('number', '')).toBeUndefined()
  })
  it('accepts any text for other types', () => {
    expect(attrInputError('text', '12kg')).toBeUndefined()
  })
})
