import type { ConfirmationOptions } from 'primevue/confirmationoptions'
import { describe, expect, it, vi } from 'vitest'
import { bindConfirm, closeGuard, confirmAction, mayClose } from './confirm'

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

describe('closeGuard', () => {
  it('ignores a second close request while the discard prompt is open', async () => {
    let answer: (v: boolean) => void = () => {}
    const ask = vi.fn(() => new Promise<boolean>((r) => (answer = r)))
    const guard = closeGuard(ask)
    const first = guard(true)
    // Esc trên hộp xác nhận cũng tới hộp thoại bên dưới: không hỏi lần nữa
    await expect(guard(true)).resolves.toBe(false)
    expect(ask).toHaveBeenCalledOnce()
    answer(false)
    await expect(first).resolves.toBe(false)
    // hỏi xong thì lần sau hỏi lại bình thường
    const again = guard(true)
    expect(ask).toHaveBeenCalledTimes(2)
    answer(true)
    await expect(again).resolves.toBe(true)
  })
})
