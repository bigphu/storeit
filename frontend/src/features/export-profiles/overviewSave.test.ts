import { describe, expect, it } from 'vitest'
import { profileChanges, visibilityOf } from './overviewSave'

const p = { name: 'Monthly', shared: false }

describe('profileChanges', () => {
  it('reads a rename without touching visibility', () => {
    expect(profileChanges(p, { name: 'Weekly' })).toEqual({ name: 'Weekly', shared: false, renamed: true, visibility: false })
  })
  it('reads a visibility change from the draft', () => {
    expect(profileChanges(p, { visibility: 'shared' })).toEqual({ name: 'Monthly', shared: true, renamed: false, visibility: true })
  })
  it('treats the saved visibility picked again as no change', () => {
    expect(profileChanges({ ...p, shared: true }, { visibility: visibilityOf({ shared: true }) }).visibility).toBe(false)
  })
})
