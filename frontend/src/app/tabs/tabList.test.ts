import { describe, expect, it } from 'vitest'
import { closeOthers, closeTab, dedupeKey, openTab, restoreTabs, type Tab, togglePin } from './tabList'

const t = (id: string, path: string, pinned = false): Tab => ({ id, path, pinned })
let n = 0
const newId = () => `n${++n}`

describe('dedupeKey', () => {
  it('lets lists open more than once, other pages once', () => {
    expect(dedupeKey('/assets?q=x')).toBeNull()
    expect(dedupeKey('/types/L/assets?attr=a:eq:1')).toBeNull()
    expect(dedupeKey('/accounts?q=lan')).toBeNull()
    expect(dedupeKey('/assets/A1')).toBe('/assets/A1')
    expect(dedupeKey('/assets/A1/edit')).toBe('/assets/A1/edit')
    expect(dedupeKey('/types/L/settings')).toBe('/types/L/settings')
    expect(dedupeKey('/account/profile?x=1')).toBe('/account/profile')
  })
})

describe('openTab', () => {
  const tabs = [t('p', '/assets?status_kind=in_use', true), t('a', '/types/L/assets'), t('b', '/assets/B')]

  it('adds a tab right after the active one, in the background or focused', () => {
    const bg = openTab(tabs, 'a', '/assets/C', { background: true }, newId)
    expect(bg.tabs.map((x) => x.path)).toEqual(['/assets?status_kind=in_use', '/types/L/assets', '/assets/C', '/assets/B'])
    expect(bg.activeId).toBe('a')
    expect(bg.opened).toBe(bg.tabs[2].id)

    const fg = openTab(tabs, 'a', '/assets/C', { background: false }, newId)
    expect(fg.activeId).toBe(fg.tabs[2].id)
  })

  it('never inserts among pinned tabs', () => {
    const r = openTab(tabs, 'p', '/assets/C', { background: true }, newId)
    expect(r.tabs[0].id).toBe('p')
    expect(r.tabs[1].path).toBe('/assets/C')
  })

  it('reuses a tab already showing the same page', () => {
    const r = openTab(tabs, 'a', '/assets/B', { background: true }, newId)
    expect(r.tabs).toBe(tabs)
    expect(r.existing).toBe('b')
    expect(openTab(tabs, 'a', '/assets/B', { background: false }, newId).activeId).toBe('b')
  })
})

describe('closeTab', () => {
  const tabs = [t('a', '/1'), t('b', '/2'), t('c', '/3')]

  it('activates the neighbour on the right, else the left', () => {
    expect(closeTab(tabs, 'b', 'b')).toEqual({ tabs: [tabs[0], tabs[2]], activeId: 'c' })
    expect(closeTab(tabs, 'c', 'c')).toEqual({ tabs: [tabs[0], tabs[1]], activeId: 'b' })
  })

  it('keeps the active tab when closing another', () => {
    expect(closeTab(tabs, 'a', 'c').activeId).toBe('a')
  })

  it('can close the last tab (the caller opens a new one)', () => {
    expect(closeTab([t('a', '/1')], 'a', 'a')).toEqual({ tabs: [], activeId: null })
  })
})

describe('pinning and closing others', () => {
  it('moves pinned tabs to the front and back', () => {
    const tabs = [t('a', '/1', true), t('b', '/2'), t('c', '/3')]
    expect(togglePin(tabs, 'c').map((x) => [x.id, x.pinned])).toEqual([
      ['a', true],
      ['c', true],
      ['b', false],
    ])
    // bỏ ghim: tab giữ chỗ, đứng đầu nhóm chưa ghim
    expect(togglePin(tabs, 'a').map((x) => [x.id, x.pinned])).toEqual([
      ['a', false],
      ['b', false],
      ['c', false],
    ])
  })

  it('closes others but keeps pinned ones', () => {
    const tabs = [t('p', '/1', true), t('a', '/2'), t('b', '/3')]
    expect(closeOthers(tabs, 'b').map((x) => x.id)).toEqual(['p', 'b'])
  })
})

describe('restoreTabs', () => {
  const saved = [t('p', '/1', true), t('a', '/2')]
  it('reopens all tabs or only pinned ones', () => {
    expect(restoreTabs(saved, true).map((x) => x.id)).toEqual(['p', 'a'])
    expect(restoreTabs(saved, false).map((x) => x.id)).toEqual(['p'])
  })
  it('drops broken entries', () => {
    expect(restoreTabs([{ id: 1 } as unknown as Tab, t('a', '/2')], true).map((x) => x.id)).toEqual(['a'])
  })
})
