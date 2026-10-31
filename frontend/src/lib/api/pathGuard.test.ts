import { describe, expect, it } from 'vitest'
import { NoQueryId } from '@/lib/queryKeyId'
import { checkPath } from './pathGuard'

describe('checkPath', () => {
  it('lets filled paths through', () => {
    expect(() => checkPath('https://x.test/api/v1/assets/01a1155b-2b55-7bbf-b0d9-50ba515951df')).not.toThrow()
    expect(() => checkPath('https://x.test/api/v1/assets?q=%7Bnot-a-path%7D')).not.toThrow()
  })
  it('stops a request whose path still has a {placeholder}', () => {
    expect(() => checkPath('https://x.test/api/v1/assets/%7BassetID%7D')).toThrow(NoQueryId)
    expect(() => checkPath('https://x.test/api/v1/borrow-tickets/{ticketID}/approve')).toThrow(NoQueryId)
  })
})
