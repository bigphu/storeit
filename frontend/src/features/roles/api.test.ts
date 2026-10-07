import { QueryClient } from '@tanstack/vue-query'
import { describe, expect, it } from 'vitest'
import { forgetDeletedRole, roleKeys } from './api'

describe('forgetDeletedRole', () => {
  it('drops the deleted role instead of refetching it, and refreshes the list', async () => {
    const qc = new QueryClient()
    qc.setQueryData(roleKeys.one('r1'), { id: 'r1' })
    qc.setQueryData(roleKeys.one('r2'), { id: 'r2' })
    qc.setQueryData(roleKeys.all, [])
    await forgetDeletedRole(qc, 'r1')
    expect(qc.getQueryData(roleKeys.one('r1'))).toBeUndefined()
    expect(qc.getQueryState(roleKeys.one('r2'))?.isInvalidated).toBe(false)
    expect(qc.getQueryState(roleKeys.all)?.isInvalidated).toBe(true)
  })
})
