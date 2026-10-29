import { describe, expect, it } from 'vitest'
import { formatDay } from '@/lib/dates'
import { activitySummary, type ActivityEntry, isLong, relativeTime } from './activity'

const entry = (patch: Partial<ActivityEntry>): ActivityEntry => ({
  id: 'e1',
  kind: 'created',
  type: 'inventory.asset_created',
  occurred_at: '2026-10-29T09:00:00Z',
  actor: { id: 'a1', name: 'Trần Thị Mai' },
  changes: [],
  ...patch,
})
const change = (label: string) => ({ field: label.toLowerCase(), label, from: 'a', to: 'b' })

describe('activitySummary', () => {
  it('names the person and what they did', () => {
    expect(activitySummary(entry({}))).toEqual({ who: 'Trần Thị Mai', text: 'created this asset' })
    expect(activitySummary(entry({ kind: 'updated', changes: [change('Status'), change('RAM')] })).text).toBe('edited 2 fields')
    expect(activitySummary(entry({ kind: 'updated', changes: [change('Status')] })).text).toBe('changed Status')
    expect(activitySummary(entry({ kind: 'restored' })).text).toBe('restored this asset')
    expect(activitySummary(entry({ kind: 'other', type: 'inventory.x' })).text).toBe('recorded inventory.x')
  })
  it('says System when no person did it, and adds the retire reason', () => {
    expect(activitySummary(entry({ kind: 'retired', reason: 'Broken screen', actor: null }))).toEqual({
      who: 'System',
      text: 'retired this asset: Broken screen',
    })
    expect(activitySummary(entry({ kind: 'retired' })).text).toBe('retired this asset')
  })
})

describe('relativeTime', () => {
  const now = new Date('2026-10-29T12:00:00Z')
  const ago = (ms: number) => new Date(now.getTime() - ms).toISOString()
  const min = 60_000
  const hour = 60 * min
  const day = 24 * hour
  it('reads like a person would say it', () => {
    expect(relativeTime(ago(20_000), now)).toBe('just now')
    expect(relativeTime(ago(5 * min), now)).toBe('5 minutes ago')
    expect(relativeTime(ago(3 * hour), now)).toBe('3 hours ago')
    expect(relativeTime(ago(day), now)).toBe('yesterday')
    expect(relativeTime(ago(10 * day), now)).toBe('10 days ago')
  })
  it('falls back to the date after a month', () => {
    const iso = ago(40 * day)
    expect(relativeTime(iso, now)).toBe(formatDay(iso))
  })
})

describe('isLong', () => {
  it('flags text worth clamping', () => {
    expect(isLong('x'.repeat(200))).toBe(true)
    expect(isLong('a\nb\nc\nd')).toBe(true)
    expect(isLong('short')).toBe(false)
    expect(isLong(null)).toBe(false)
  })
})

describe('activitySummary for replacements', () => {
  const rel = (tag: string, role?: 'old' | 'new') => ({ id: 'x', tag, ...(role ? { role } : {}) })
  it('names the other asset', () => {
    expect(activitySummary(entry({ kind: 'replaced', related: rel('LAP-0042'), reason: 'screen broken' })).text).toBe(
      'replaced this asset with LAP-0042: screen broken',
    )
    expect(activitySummary(entry({ kind: 'replaces', related: rel('LAP-0017') })).text).toBe('recorded that this asset replaces LAP-0017')
    expect(activitySummary(entry({ kind: 'replacement_undone', related: rel('LAP-0042', 'old') })).text).toBe('undid the replacement by LAP-0042')
    expect(activitySummary(entry({ kind: 'replacement_undone', related: rel('LAP-0017', 'new') })).text).toBe('undid the replacement of LAP-0017')
  })
})
