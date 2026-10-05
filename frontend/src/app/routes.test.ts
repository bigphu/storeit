import { describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter, type RouteRecordRaw } from 'vue-router'
import { routes } from './routes'

const router = createRouter({ history: createMemoryHistory(), routes })
const isPublic = (path: string) => router.resolve(path).matched.some((r) => r.meta.public)

describe('routes', () => {
  it('sends / into the app (which redirects to /assets), not an empty public layout', () => {
    const r = router.resolve('/')
    expect(isPublic('/')).toBe(false)
    expect(r.matched.at(-1)?.redirect).toBe('/assets')
  })

  it('keeps the auth pages public', () => {
    for (const p of ['/login', '/forgot-password', '/accept-invite', '/reset-password']) {
      expect(isPublic(p), p).toBe(true)
    }
  })

  it('keeps app pages private and names them', () => {
    expect(isPublic('/assets')).toBe(false)
    expect(router.resolve('/assets/123/edit').name).toBe('asset-edit')
    expect(router.resolve('/accounts/abc').name).toBe('account')
    expect(router.resolve('/nope/at/all').name).toBe('not-found')
    expect(isPublic('/nope/at/all')).toBe(false)
  })
})

// stubbed: cùng bảng route, component thay bằng component rỗng
const Stub = { render: () => null }
function stubbed(list: RouteRecordRaw[]): RouteRecordRaw[] {
  return list.map((r) => {
    const out = { ...r } as RouteRecordRaw & { component?: unknown; children?: RouteRecordRaw[] }
    if ('component' in r && r.component) out.component = Stub
    if (r.children) out.children = stubbed(r.children)
    return out as RouteRecordRaw
  })
}

describe('type and account routes', () => {
  it('scopes asset lists, forms and settings by type', () => {
    expect(router.resolve('/types/L/assets').name).toBe('type-assets')
    expect(router.resolve('/types/L/assets').params.typeId).toBe('L')
    expect(router.resolve('/types/L/assets/new').name).toBe('type-asset-new')
    expect(router.resolve('/types/L/settings').name).toBe('type-settings')
    expect(router.resolve('/types').name).toBe('types')
  })

  it('redirects old links', async () => {
    // điều hướng thật nhưng không tải trang (chỉ kiểm tra redirect, nhanh và ổn định)
    const r = createRouter({ history: createMemoryHistory(), routes: stubbed(routes) })
    await r.push('/asset-types/X')
    expect(r.currentRoute.value.fullPath).toBe('/types/X/settings')
    await r.push('/asset-types')
    expect(r.currentRoute.value.fullPath).toBe('/types')
    await r.push('/assets?type_id=L&q=dell')
    expect(r.currentRoute.value.fullPath).toBe('/types/L/assets?q=dell')
    await r.push('/account')
    expect(r.currentRoute.value.fullPath).toBe('/account/profile')
  })

  it('has one account settings page with sections', () => {
    for (const s of ['profile', 'password', 'preferences']) {
      const r = router.resolve(`/account/${s}`)
      expect(r.name).toBe('account-settings')
      expect(r.params.section).toBe(s)
    }
    expect(router.resolve('/account/bogus').name).toBe('not-found')
  })
})
