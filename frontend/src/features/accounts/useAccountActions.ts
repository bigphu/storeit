// Hành động trên một tài khoản, dùng chung cho danh sách và trang tài khoản: gửi lại
// lời mời, gửi link đặt lại mật khẩu, khoá, mở khoá, đăng xuất mọi nơi. Hỏi lại bằng
// ConfirmDialog; lỗi đã hiện bằng toast mặc định của mutation.
import { useConfirm } from 'primevue/useconfirm'
import type { Account } from '@/lib/api/types'
import { useSession } from '@/lib/auth/session'
import { notify } from '@/lib/notify'
import {
  useDisableAccount,
  useEnableAccount,
  useResendInvitation,
  useSendPasswordReset,
  useSignOutAccount,
} from './api'

type Target = Pick<Account, 'id' | 'name' | 'email' | 'status'>

export function useAccountActions() {
  const confirm = useConfirm()
  const session = useSession()
  const disable = useDisableAccount()
  const enable = useEnableAccount()
  const resend = useResendInvitation()
  const reset = useSendPasswordReset()
  const signOut = useSignOutAccount()

  function ask(header: string, message: string, acceptLabel: string, danger: boolean, run: () => Promise<unknown>) {
    confirm.require({
      header,
      message,
      acceptLabel,
      rejectLabel: 'Cancel',
      acceptProps: danger ? { severity: 'danger' } : undefined,
      accept: () => run().catch(() => {}),
    })
  }

  const isSelf = (a: Target) => a.id === session.me?.account.id

  return {
    isSelf,
    resendInvitation(a: Target) {
      ask('Resend invitation', `Send a new invitation to ${a.email}? The previous link stops working.`, 'Send invitation', false, () =>
        resend.mutateAsync(a.id).then(() => notify.success('Invitation sent.')),
      )
    },
    sendReset(a: Target) {
      ask('Send reset link', `Email a password reset link to ${a.email}? It works for one hour.`, 'Send link', false, () =>
        reset.mutateAsync(a.id).then(() => notify.success('Reset link sent.')),
      )
    },
    disable(a: Target) {
      ask('Disable account', `Disable ${a.name}? They are signed out everywhere and their links stop working.`, 'Disable', true, () =>
        disable.mutateAsync(a.id).then(() => notify.success('Account disabled.')),
      )
    },
    enable(a: Target) {
      ask('Enable account', `Enable ${a.name}? They get back every permission their roles give.`, 'Enable', false, () =>
        enable.mutateAsync(a.id).then(() => notify.success('Account enabled.')),
      )
    },
    signOutEverywhere(a: Target) {
      ask('Sign out everywhere', `Sign ${a.name} out on every device? They need to sign in again.`, 'Sign out', true, () =>
        signOut.mutateAsync(a.id).then((r) =>
          notify.success(r.revoked ? `Ended ${r.revoked} session${r.revoked === 1 ? '' : 's'}.` : 'No sessions were open.'),
        ),
      )
    },
  }
}
