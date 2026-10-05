import { describe, expect, it } from 'vitest'
import { initials, inviteNote, relativeTime } from './people'

const now = new Date('2026-10-04T12:00:00Z').getTime()
const at = (hours: number) => new Date(now + hours * 3600_000).toISOString()

describe('initials', () => {
  it('takes the first and last words', () => {
    expect(initials('Trần Minh Quân')).toBe('TQ')
    expect(initials('  ada  ')).toBe('A')
    expect(initials('')).toBe('?')
  })
})

describe('relativeTime', () => {
  it('picks the largest fitting unit', () => {
    expect(relativeTime(at(48), now)).toBe('in 2 days')
    expect(relativeTime(at(-24), now)).toBe('yesterday')
    expect(relativeTime(at(-3), now)).toBe('3 hours ago')
    expect(relativeTime(at(0), now)).toBe('now')
  })
})

describe('inviteNote', () => {
  it('says whether the link still works', () => {
    expect(inviteNote(at(48), now)).toBe('Invite expires in 2 days')
    expect(inviteNote(at(-24), now)).toBe('Invite expired yesterday')
    expect(inviteNote(undefined, now)).toBeUndefined()
  })
})
