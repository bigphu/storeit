import { describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
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
