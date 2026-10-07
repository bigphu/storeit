import { describe, expect, it } from 'vitest'
import { ICONS } from './icons'

describe('icons', () => {
  it('has every icon the spec names, each a drawable path', () => {
    for (const name of ['tag', 'trash', 'mail', 'logout', 'alert', 'tick', 'user-plus', 'sitemap', 'shield', 'file', 'sliders', 'box'] as const) {
      expect(ICONS[name].paths.length).toBeGreaterThan(0)
      for (const d of ICONS[name].paths) expect(d).toMatch(/^M/)
    }
  })
})
