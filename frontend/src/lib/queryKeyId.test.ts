import { describe, expect, it } from 'vitest'
import { keyId, NoQueryId } from './queryKeyId'

describe('keyId', () => {
  it('reads the id from its place in the query key', () => {
    expect(keyId(['assets', 'a1'], 1)).toBe('a1')
    expect(keyId(['borrowing', 'ticket', 't1'], 2)).toBe('t1')
  })
  it('refuses a key without an id, so no request goes out as /assets/{assetID}', () => {
    expect(() => keyId(['assets', ''], 1)).toThrow(NoQueryId)
    expect(() => keyId(['assets'], 1)).toThrow()
    expect(() => keyId(['assets', undefined], 1)).toThrow()
  })
})
