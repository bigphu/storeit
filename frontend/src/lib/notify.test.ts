import { describe, expect, it, vi } from 'vitest'
import { type Notice, notify, onNotice } from './notify'

describe('notify', () => {
  it('passes an optional action to listeners', () => {
    const seen: Notice[] = []
    const off = onNotice((n) => seen.push(n))
    const run = vi.fn()
    notify.success('Downloaded a.xlsx.', { label: 'What’s inside', run })
    notify.info('Plain')
    off()
    expect(seen[0].action?.label).toBe('What’s inside')
    seen[0].action?.run()
    expect(run).toHaveBeenCalledOnce()
    expect(seen[1].action).toBeUndefined()
  })
})
