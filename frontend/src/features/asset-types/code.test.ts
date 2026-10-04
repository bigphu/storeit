import { describe, expect, it } from 'vitest'
import { codeFromName, codeMark } from './code'

describe('codeFromName', () => {
  it('uppercases and joins words with underscores', () => {
    expect(codeFromName('Network gear')).toBe('NETWORK_GEAR')
    expect(codeFromName('  Laptop  ')).toBe('LAPTOP')
  })

  it('drops Vietnamese diacritics, including đ', () => {
    expect(codeFromName('Điện thoại di động')).toBe('DIEN_THOAI_DI_DONG')
  })

  it('collapses symbols and keeps it within 32 characters', () => {
    expect(codeFromName('Printer / Scanner (A3)')).toBe('PRINTER_SCANNER_A3')
    const long = codeFromName('a'.repeat(31) + ' bbb')
    expect(long.length).toBeLessThanOrEqual(32)
    expect(long.endsWith('_')).toBe(false)
  })
})

describe('codeMark', () => {
  it('takes the first two letters or digits', () => {
    expect(codeMark('NETWORK_GEAR')).toBe('NE')
    expect(codeMark('_X')).toBe('X')
  })
})
