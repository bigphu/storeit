import { describe, expect, it } from 'vitest'
import { ApiError, describeError, unwrap } from './errors'

const res = (status: number, headers: Record<string, string> = {}) => new Response(null, { status, headers })

describe('unwrap', () => {
  it('returns data on success', async () => {
    await expect(unwrap(Promise.resolve({ data: { id: 1 }, response: res(200) }))).resolves.toEqual({ id: 1 })
  })

  it('returns undefined for 204', async () => {
    await expect(unwrap(Promise.resolve({ data: undefined, response: res(204) }))).resolves.toBeUndefined()
  })

  it('throws ApiError with problem fields', async () => {
    const problem = {
      type: '/errors/invalid-attribute-values',
      title: 'Invalid attribute values',
      status: 422,
      errors: [
        { field: 'attributes.ram_gb', detail: 'must be a number' },
        { field: 'attributes.ram_gb', detail: 'second message ignored' },
        { field: 'tag', detail: 'is taken' },
      ],
    }
    const err = (await unwrap(Promise.resolve({ error: problem, response: res(422) })).catch((e: unknown) => e)) as ApiError
    expect(err).toBeInstanceOf(ApiError)
    expect(err.type).toBe('/errors/invalid-attribute-values')
    expect(err.status).toBe(422)
    expect(err.fields).toEqual({ 'attributes.ram_gb': 'must be a number', tag: 'is taken' })
  })

  it('copes with a body that is not a problem', async () => {
    const err = (await unwrap(Promise.resolve({ error: 'Bad Gateway', response: res(502) })).catch((e: unknown) => e)) as ApiError
    expect(err).toBeInstanceOf(ApiError)
    expect(err.status).toBe(502)
    expect(err.type).toBe('about:blank')
    expect(err.fields).toEqual({})
  })
})

describe('describeError', () => {
  it('uses detail, then title', () => {
    expect(describeError(new ApiError({ type: 't', title: 'Title', status: 409, detail: 'Detail' }))).toBe('Detail')
    expect(describeError(new ApiError({ type: 't', title: 'Title', status: 409 }))).toBe('Title')
  })

  it('explains 403 and 429', () => {
    expect(describeError(new ApiError({ type: 't', title: 'Forbidden', status: 403 }))).toBe(
      "You don't have permission to do that.",
    )
    expect(describeError(new ApiError({ type: 't', title: 'Too many requests', status: 429 }, 12))).toBe(
      'Too many attempts. Try again in 12 s.',
    )
  })

  it('handles network errors', () => {
    expect(describeError(new TypeError('Failed to fetch'))).toBe('Cannot reach the server.')
  })
})
