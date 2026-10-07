import { describe, expect, it } from 'vitest'
import {
  changeCount, changesOf, clearTab, discardTab, emptyDraft, isDirty, listOf, previousOf, restoreTab, setEdit, setList, valueOf,
} from './detailDraft'

describe('detailDraft', () => {
  it('counts edits per tab and drops an edit set back to the saved value', () => {
    const d = emptyDraft()
    setEdit(d, 'overview', 'name', 'Kế toán 2', 'Kế toán')
    setEdit(d, 'overview', 'description', 'x', '')
    expect(changeCount(d, 'overview')).toBe(2)
    expect(valueOf(d, 'overview', 'name', 'Kế toán')).toBe('Kế toán 2')
    setEdit(d, 'overview', 'name', 'Kế toán', 'Kế toán')
    expect(changeCount(d, 'overview')).toBe(1)
    expect(valueOf(d, 'overview', 'name', 'Kế toán')).toBe('Kế toán')
    expect(isDirty(d)).toBe(true)
    expect(changesOf(d, 'overview')).toEqual({ description: 'x' })
  })

  it('discards a tab and Undo merges the edits back without losing other tabs', () => {
    const d = emptyDraft()
    setEdit(d, 'overview', 'name', 'B', 'A')
    const removed = discardTab(d, 'overview')
    expect(isDirty(d)).toBe(false)
    setEdit(d, 'permissions', 'x', true, false)
    setEdit(d, 'overview', 'description', 'd', '')
    restoreTab(d, 'overview', removed)
    expect(changesOf(d, 'overview')).toEqual({ description: 'd', name: 'B' })
    expect(changeCount(d, 'permissions')).toBe(1)
    clearTab(d, 'overview')
    expect(changeCount(d, 'overview')).toBe(0)
  })

  it('gives the saved values of the changed keys for Undo', () => {
    expect(previousOf({ name: 'A', description: 'old', code: 'LAP' }, { name: 'B', description: 'new' })).toEqual({ name: 'A', description: 'old' })
  })

  it('tracks tick lists key by key', () => {
    const d = emptyDraft()
    const saved = ['assets.read']
    // Manage kéo theo View: cả hai cùng đổi
    setList(d, 'permissions', saved, ['assets.read', 'types.read', 'types.manage'])
    expect(changeCount(d, 'permissions')).toBe(2)
    expect(listOf(d, 'permissions', saved, ['assets.read', 'types.read', 'types.manage', 'roles.read'])).toEqual(['assets.read', 'types.read', 'types.manage'])
    setList(d, 'permissions', saved, ['assets.read'])
    expect(changeCount(d, 'permissions')).toBe(0)
  })
})
