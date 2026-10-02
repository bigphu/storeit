// Access token chỉ nằm trong bộ nhớ; refresh token ở cookie HttpOnly mà JS không đọc được.
// authFetch gắn bearer, gặp 401 thì refresh một lần rồi gửi lại request một lần.

export interface TokenDeps {
  // Gọi POST /auth/refresh; trả access token mới, null khi phiên đã hết (401)
  refresh: () => Promise<string | null>
  // Khoá dùng chung giữa các tab (navigator.locks): mỗi lúc chỉ một refresh, như
  // identity.md yêu cầu, để trình duyệt không giữ lại cookie cũ đã dùng
  withLock: <T>(fn: () => Promise<T>) => Promise<T>
  // Refresh thất bại: phiên kết thúc
  onSignedOut: () => void
  // Refresh thành công, token mới đã gắn: nạp lại quyền (GET /me)
  onRefreshed?: () => void
}

export type BaseFetch = (request: Request) => Promise<Response>

// 401 của các endpoint này là kết quả thật, không phải token hết hạn
const publicAuthPaths = ['/auth/login', '/auth/refresh', '/auth/logout', '/auth/password/forgot', '/auth/password/set']

function isPublicAuth(url: string): boolean {
  const path = new URL(url).pathname
  return publicAuthPaths.some((p) => path.endsWith(p))
}

function withBearer(request: Request, token: string | null): Request {
  if (!token) return request
  const r = new Request(request)
  r.headers.set('Authorization', `Bearer ${token}`)
  return r
}

export function createTokens(deps: TokenDeps) {
  let token: string | null = null
  let inflight: Promise<string | null> | null = null

  // refresh: các lời gọi cùng lúc trong một tab dùng chung một promise
  function refresh(): Promise<string | null> {
    inflight ??= deps
      .withLock(deps.refresh)
      .then((t) => {
        token = t
        if (t) deps.onRefreshed?.()
        return t
      })
      .finally(() => {
        inflight = null
      })
    return inflight
  }

  async function authFetch(request: Request, base: BaseFetch = fetch): Promise<Response> {
    const retry = request.clone() // body chỉ đọc được một lần: giữ bản sao để gửi lại
    const sent = token
    const res = await base(withBearer(request, sent))
    if (res.status !== 401 || isPublicAuth(request.url)) return res

    // một request khác có thể đã refresh trong lúc request này chạy
    const fresh = token !== null && token !== sent ? token : await refresh()
    if (!fresh) {
      deps.onSignedOut()
      return res
    }
    return base(withBearer(retry, fresh))
  }

  return {
    authFetch,
    refresh,
    get: () => token,
    set: (t: string | null) => {
      token = t
    },
  }
}

export type Tokens = ReturnType<typeof createTokens>
