import { describe, expect, it, vi } from 'vitest'
import { createTokens } from './tokens'

const ok = (body = '{}') => new Response(body, { status: 200 })
const unauthorized = () => new Response('{}', { status: 401 })

function setup(refreshResults: (string | null)[]) {
  const refresh = vi.fn(async () => refreshResults.shift() ?? null)
  const onSignedOut = vi.fn()
  const tokens = createTokens({ refresh, withLock: (fn) => fn(), onSignedOut })
  return { tokens, refresh, onSignedOut }
}

describe('authFetch', () => {
  it('sends the bearer token', async () => {
    const { tokens } = setup([])
    tokens.set('t1')
    const base = vi.fn(async (_: Request) => ok())
    await tokens.authFetch(new Request('http://x/api/v1/assets'), base)
    expect(base.mock.calls[0][0].headers.get('Authorization')).toBe('Bearer t1')
  })

  it('refreshes once on 401 and retries with the new token and the same body', async () => {
    const { tokens, refresh } = setup(['t2'])
    tokens.set('t1')
    const seen: { auth: string | null; body: string }[] = []
    const base = vi.fn(async (r: Request) => {
      seen.push({ auth: r.headers.get('Authorization'), body: await r.text() })
      return seen.length === 1 ? unauthorized() : ok()
    })
    const res = await tokens.authFetch(new Request('http://x/api/v1/assets', { method: 'POST', body: '{"a":1}' }), base)
    expect(res.status).toBe(200)
    expect(refresh).toHaveBeenCalledTimes(1)
    expect(seen).toEqual([
      { auth: 'Bearer t1', body: '{"a":1}' },
      { auth: 'Bearer t2', body: '{"a":1}' },
    ])
    expect(tokens.get()).toBe('t2')
  })

  it('shares one refresh between concurrent 401s', async () => {
    const { tokens, refresh } = setup(['t2'])
    tokens.set('t1')
    const base = vi.fn(async (r: Request) => (r.headers.get('Authorization') === 'Bearer t1' ? unauthorized() : ok()))
    const results = await Promise.all([
      tokens.authFetch(new Request('http://x/api/v1/a'), base),
      tokens.authFetch(new Request('http://x/api/v1/b'), base),
      tokens.authFetch(new Request('http://x/api/v1/c'), base),
    ])
    expect(results.map((r) => r.status)).toEqual([200, 200, 200])
    expect(refresh).toHaveBeenCalledTimes(1)
  })

  it('signs out when the refresh fails and returns the original 401', async () => {
    const { tokens, onSignedOut } = setup([null])
    tokens.set('t1')
    const base = vi.fn(async () => unauthorized())
    const res = await tokens.authFetch(new Request('http://x/api/v1/assets'), base)
    expect(res.status).toBe(401)
    expect(base).toHaveBeenCalledTimes(1)
    expect(onSignedOut).toHaveBeenCalledTimes(1)
    expect(tokens.get()).toBeNull()
  })

  it('does not refresh for 401 from public auth endpoints', async () => {
    const { tokens, refresh } = setup(['t2'])
    const base = vi.fn(async () => unauthorized())
    const res = await tokens.authFetch(new Request('http://x/api/v1/auth/login', { method: 'POST' }), base)
    expect(res.status).toBe(401)
    expect(refresh).not.toHaveBeenCalled()
  })

  it('calls onRefreshed after the new token is in place', async () => {
    const refresh = vi.fn(async () => 't2')
    let seen: string | null = null
    const tokens = createTokens({
      refresh,
      withLock: (fn) => fn(),
      onSignedOut: () => {},
      onRefreshed: () => {
        seen = tokens.get()
      },
    })
    await tokens.refresh()
    expect(seen).toBe('t2')
  })

  it('retries once only', async () => {
    const { tokens, refresh } = setup(['t2', 't3'])
    tokens.set('t1')
    const base = vi.fn(async () => unauthorized())
    const res = await tokens.authFetch(new Request('http://x/api/v1/assets'), base)
    expect(res.status).toBe(401)
    expect(base).toHaveBeenCalledTimes(2)
    expect(refresh).toHaveBeenCalledTimes(1)
  })
})
