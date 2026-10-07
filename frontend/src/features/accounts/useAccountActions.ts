// Hành động trên một tài khoản, dùng chung cho danh sách, ngăn kéo sửa nhanh và trang tài
// khoản. Gửi email, đăng xuất mọi nơi và khoá tài khoản hỏi trước (khoá đá người đó ra);
// khoá / mở khoá vẫn có Undo
import type { Account } from '@/lib/api/types'
import { runAction } from '@/lib/actions'
import { useSession } from '@/lib/auth/session'
import { confirmAction } from '@/lib/confirm'
import {
  useDisableAccount,
  useEnableAccount,
  useResendInvitation,
  useSendPasswordReset,
  useSignOutAccount,
} from './api'

type Target = Pick<Account, 'id' | 'name' | 'email' | 'status'>

export function useAccountActions() {
  const session = useSession()
  const disable = useDisableAccount()
  const enable = useEnableAccount()
  const resend = useResendInvitation()
  const reset = useSendPasswordReset()
  const signOut = useSignOutAccount()

  const isSelf = (a: Target) => a.id === session.me?.account.id

  return {
    isSelf,
    async resendInvitation(a: Target) {
      const ok = await confirmAction({
        title: `Resend the invitation to ${a.email}?`,
        body: 'A new link is emailed. The previous link stops working.',
        action: 'Send invitation',
        danger: false,
        icon: 'mail',
      })
      if (ok) await runAction({ run: () => resend.mutateAsync(a.id), done: `Invitation sent to ${a.email}.`, failed: `Couldn't send the invitation to ${a.email}.` })
    },
    async sendReset(a: Target) {
      const ok = await confirmAction({
        title: `Send ${a.name} a password reset link?`,
        body: `The link goes to ${a.email} and works for one hour.`,
        action: 'Send link',
        danger: false,
        icon: 'mail',
      })
      if (ok) await runAction({ run: () => reset.mutateAsync(a.id), done: `Reset link sent to ${a.email}.`, failed: `Couldn't send the reset link to ${a.email}.` })
    },
    async disable(a: Target) {
      const ok = await confirmAction({
        title: `Disable ${a.name}?`,
        body: 'They are signed out within 15 minutes and can’t sign in until the account is enabled again.',
        action: 'Disable account',
        danger: false,
        warn: true,
        icon: 'alert',
      })
      if (!ok) return false
      return runAction({
        run: () => disable.mutateAsync(a.id),
        done: `${a.name} disabled.`,
        failed: `Couldn't disable ${a.name}.`,
        undo: () => enable.mutateAsync(a.id),
        undone: `${a.name} enabled again.`,
        undoFailed: `Couldn't enable ${a.name} again. The account is still disabled.`,
      })
    },
    enable(a: Target) {
      return runAction({
        run: () => enable.mutateAsync(a.id),
        done: `${a.name} enabled.`,
        failed: `Couldn't enable ${a.name}.`,
        undo: () => disable.mutateAsync(a.id),
        undone: `${a.name} disabled again.`,
        undoFailed: `Couldn't disable ${a.name} again. The account stays enabled.`,
      })
    },
    async signOutEverywhere(a: Target) {
      const ok = await confirmAction({
        title: `Sign ${a.name} out everywhere?`,
        body: 'Every device they are signed in on is signed out. They need their password to come back.',
        action: 'Sign out everywhere',
        danger: false,
        icon: 'logout',
      })
      if (!ok) return
      await runAction({
        run: () => signOut.mutateAsync(a.id),
        done: (r) => (r.revoked ? `${a.name} signed out on ${r.revoked} device${r.revoked === 1 ? '' : 's'}.` : `${a.name} had no open sessions.`),
        failed: `Couldn't sign ${a.name} out.`,
      })
    },
  }
}
