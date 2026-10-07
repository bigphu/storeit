import type { ConfirmationOptions } from 'primevue/confirmationoptions'
import { describe, expect, it, vi } from 'vitest'
import { bindConfirm, confirmAction, mayClose } from './confirm'

function fake() {
  let last: ConfirmationOptions | undefined
  bindConfirm((o) => (last = o))
  return () => last!
}

const opts = { title: 'Sign Hùng out everywhere?', body: 'Every device is signed out.', action: 'Sign out everywhere', danger: false, icon: 'logout' } as const

describe('confirmAction', () => {
  it('resolves true on accept and passes the view to the template', async () => {
    const last = fake()
    const p = confirmAction({ ...opts, impact: ['3 devices'] })
    expect((last() as unknown as { view: typeof opts }).view.action).toBe('Sign out everywhere')
    last().accept!()
    await expect(p).resolves.toBe(true)
  })
  it('resolves false on reject or hide, and only once', async () => {
    const last = fake()
    const p = confirmAction(opts)
    last().onHide!()
    last().accept!()
    await expect(p).resolves.toBe(false)
    const q = confirmAction(opts)
    last().reject!()
    await expect(q).resolves.toBe(false)
  })
})

describe('mayClose', () => {
  it('closes clean forms without asking, asks for dirty ones', async () => {
    const ask = vi.fn().mockResolvedValue(false)
    await expect(mayClose(false, ask)).resolves.toBe(true)
    expect(ask).not.toHaveBeenCalled()
    await expect(mayClose(true, ask)).resolves.toBe(false)
    ask.mockResolvedValue(true)
    await expect(mayClose(true, ask)).resolves.toBe(true)
  })
})
