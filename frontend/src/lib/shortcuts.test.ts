import { describe, expect, it } from 'vitest'
import { GROUP_ORDER, SHORTCUTS, shortcutGroups } from './shortcuts'

describe('shortcuts', () => {
  it('lists every group in order and none is empty', () => {
    const groups = shortcutGroups()
    expect(groups.map((g) => g.group)).toEqual(GROUP_ORDER)
    for (const g of groups) expect(g.items.length).toBeGreaterThan(0)
  })

  it('lists each key once within a group', () => {
    for (const g of shortcutGroups()) {
      const keys = g.items.map((s) => s.keys.join(' '))
      expect(new Set(keys).size).toBe(keys.length)
    }
  })

  it('includes the new list keys and sideways scrolling', () => {
    const lists = SHORTCUTS.filter((s) => s.group === 'Lists').map((s) => s.keys.join(' '))
    expect(lists).toEqual(expect.arrayContaining(['J K', '/', 'N', 'F', 'Shift + wheel']))
  })

  it('quick edit uses J/K and Ctrl Enter', () => {
    const quick = SHORTCUTS.filter((s) => s.group === 'Quick edit').map((s) => s.keys.join(' '))
    expect(quick).toEqual(['J K', 'Ctrl S', 'Ctrl Enter'])
  })
})
