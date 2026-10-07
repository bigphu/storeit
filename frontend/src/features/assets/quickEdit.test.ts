import { describe, expect, it } from 'vitest'
import type { AssetDetail } from '@/lib/api/types'
import { quickAssetBody } from './quickEdit'
import { assetBodyOf } from './values'

const a = {
  id: 'x', tag: 'CHR-0023', name: 'Ghế họp', description: 'Broken armrest.', version: 7,
  asset_type: { id: 't1' }, status: { id: 's-repair' }, location_id: 'l1', purchase_date: '2026-03-25',
  attributes: [{ key: 'color', label: 'Màu', data_type: 'text', value: 'Xám' }],
} as unknown as AssetDetail

describe('quickAssetBody', () => {
  it('changes only what was edited and keeps every other field for the full replace', () => {
    const body = quickAssetBody(a, { status_id: 's-available' })
    expect(body).toEqual({ ...assetBodyOf(a), status_id: 's-available' })
    expect(body.description).toBe('Broken armrest.')
    expect(body.attributes).toEqual({ color: 'Xám' })
  })
})
