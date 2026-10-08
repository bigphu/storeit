import { describe, expect, it } from 'vitest'
import type { RouteLocationNormalized } from 'vue-router'
import { parseAssetQuery, toApiParams } from '@/features/assets/listQuery'
import { defaultPrefs } from '@/lib/preferences'
import { assetListParams } from './prefetch'

const route = (name: string, query: Record<string, string>, params: Record<string, string> = {}) =>
  ({ name, query, params }) as unknown as RouteLocationNormalized

describe('assetListParams', () => {
  it('builds the same params as the asset list page, with the default page size', () => {
    const to = route('assets', { q: 'lap', page: '2', status: 's1' })
    expect(assetListParams(to, defaultPrefs())).toEqual(toApiParams(parseAssetQuery(to.query), 25))
  })
  it('uses the type from the path and the size saved for that type', () => {
    const to = route('type-assets', { sort: '-name' }, { typeId: 't9' })
    const prefs = { ...defaultPrefs(), tableSizes: { 'assets:t9': 100 } }
    expect(assetListParams(to, prefs)).toEqual(toApiParams({ ...parseAssetQuery(to.query), typeId: 't9' }, 100))
  })
  it('is undefined for pages that are not an asset list', () => {
    expect(assetListParams(route('accounts', {}), defaultPrefs())).toBeUndefined()
  })
})
