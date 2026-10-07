import { describe, expect, it } from 'vitest'
import { ref } from 'vue'
import { useDirty } from './forms'

describe('useDirty', () => {
  it('compares the form with its state at the last reset', () => {
    const name = ref('a')
    const form = useDirty(() => ({ name: name.value }))
    form.reset()
    expect(form.dirty.value).toBe(false)
    name.value = 'b'
    expect(form.dirty.value).toBe(true)
    name.value = 'a'
    expect(form.dirty.value).toBe(false)
    name.value = 'c'
    form.reset()
    expect(form.dirty.value).toBe(false)
  })
})
