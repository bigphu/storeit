// Lỗi API dạng problem (RFC 9457) của backend: type, title, status, detail, errors[].

export interface Problem {
  type: string
  title: string
  status: number
  detail?: string
  errors?: { field: string; detail: string }[]
}

export class ApiError extends Error {
  readonly type: string
  readonly title: string
  readonly status: number
  readonly detail?: string
  // field -> thông báo đầu tiên của field đó (ví dụ "attributes.ram_gb", "tag")
  readonly fields: Record<string, string>
  // Retry-After (giây) của 429
  readonly retryAfter?: number

  constructor(problem: Problem, retryAfter?: number) {
    super(problem.detail || problem.title)
    this.name = 'ApiError'
    this.type = problem.type
    this.title = problem.title
    this.status = problem.status
    this.detail = problem.detail
    this.retryAfter = retryAfter
    this.fields = {}
    for (const e of problem.errors ?? []) {
      this.fields[e.field] ??= e.detail
    }
  }
}

function isProblem(v: unknown): v is Problem {
  return typeof v === 'object' && v !== null && 'type' in v && 'title' in v
}

// unwrap: kết quả của openapi-fetch -> data, hoặc ném ApiError
export async function unwrap<T>(
  call: Promise<{ data?: T; error?: unknown; response: Response }>,
): Promise<T> {
  const { data, error, response } = await call
  if (response.ok) return data as T
  const problem: Problem = isProblem(error)
    ? { ...error, status: error.status ?? response.status }
    : { type: 'about:blank', title: response.statusText || `HTTP ${response.status}`, status: response.status }
  const retry = Number(response.headers.get('Retry-After'))
  throw new ApiError(problem, Number.isFinite(retry) && retry > 0 ? retry : undefined)
}

// describeError: một câu cho toast hoặc phần lỗi chung của form
export function describeError(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 403) return "You don't have permission to do that."
    if (err.status === 429) {
      return err.retryAfter ? `Too many attempts. Try again in ${err.retryAfter} s.` : 'Too many attempts. Try again later.'
    }
    return err.detail || err.title
  }
  if (err instanceof TypeError) return 'Cannot reach the server.'
  return err instanceof Error ? err.message : 'Something went wrong.'
}

export function isApiError(err: unknown, type?: string): err is ApiError {
  return err instanceof ApiError && (type === undefined || err.type === type)
}
