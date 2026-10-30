// Hành động trên phiếu mượn, dùng chung cho bảng phiếu và trang phiếu: duyệt (ApproveDialog),
// từ chối (RejectDialog), giao, huỷ (phiếu của mình: hỏi trước; của người khác: RejectDialog
// cancel, lý do tuỳ chọn). Không có Undo: giữ chỗ và khoản mượn gửi
// thư và là hồ sơ, đảo lại là huỷ rõ ràng (spec)
import type { MenuItem } from 'primevue/menuitem'
import { computed, ref } from 'vue'
import type { BorrowTicket } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { confirmAction } from '@/lib/confirm'
import { describeError } from '@/lib/errors'
import { notify } from '@/lib/notify'
import { useCancelTicket, useHandOver } from './api'

export function useTicketActions() {
  const session = useSession()
  const canManage = computed(() => session.can(Perm.BorrowManage))
  const handOver = useHandOver()
  const cancel = useCancelTicket()

  // approveTarget/rejectTarget + *Open gắn vào ApproveDialog / RejectDialog
  const approveTarget = ref<BorrowTicket | null>(null)
  const approveOpen = ref(false)
  const rejectTarget = ref<BorrowTicket | null>(null)
  const rejectOpen = ref(false)
  // rejectCancel: hộp RejectDialog đang huỷ (người quản lý) chứ không từ chối
  const rejectCancel = ref(false)

  const isOwn = (t: BorrowTicket) => t.requester.id === session.me?.account.id
  const can = {
    approve: (t: BorrowTicket) => canManage.value && t.state === 'waiting',
    reject: (t: BorrowTicket) => canManage.value && t.state === 'waiting',
    handOver: (t: BorrowTicket) => canManage.value && t.state === 'approved',
    // người gửi: phiếu của mình khi đang chờ hay đã duyệt; người quản lý: phiếu đã duyệt
    cancel: (t: BorrowTicket) =>
      (isOwn(t) && (t.state === 'waiting' || t.state === 'approved')) || (canManage.value && t.state === 'approved'),
  }

  function askApprove(t: BorrowTicket) {
    approveTarget.value = t
    approveOpen.value = true
  }
  function askReject(t: BorrowTicket, cancel = false) {
    rejectTarget.value = t
    rejectCancel.value = cancel
    rejectOpen.value = true
  }

  async function doHandOver(t: BorrowTicket) {
    try {
      const r = await handOver.mutateAsync({ id: t.id, version: t.version })
      notify.success(`${r.asset?.tag ?? 'Asset'} handed over to ${r.requester.name}.`)
    } catch (err) {
      notify.error(`Couldn't hand over ticket #${t.number}.`, { detail: describeError(err) })
    }
  }

  async function doCancel(t: BorrowTicket) {
    // phiếu của người khác: người gửi nhận thư, người quản lý ghi lý do nếu muốn
    if (!isOwn(t)) return askReject(t, true)
    const reserved = t.state === 'approved' && t.asset
    const ok = await confirmAction({
      title: `Cancel ticket #${t.number}?`,
      body: 'The request is withdrawn.',
      impact: reserved ? [`${t.asset!.tag} is no longer reserved and becomes available.`] : undefined,
      action: 'Cancel ticket',
      danger: false,
      icon: 'alert',
    })
    if (!ok) return
    try {
      await cancel.mutateAsync({ id: t.id, version: t.version })
      notify.success(`Ticket #${t.number} cancelled.`)
    } catch (err) {
      notify.error(`Couldn't cancel ticket #${t.number}.`, { detail: describeError(err) })
    }
  }

  // menuItems: hành động của một phiếu cho menu dòng (sau Open)
  function menuItems(t: BorrowTicket): MenuItem[] {
    const items: MenuItem[] = []
    if (can.approve(t)) items.push({ label: 'Approve…', icon: 'pi pi-check', command: () => askApprove(t) })
    if (can.reject(t)) items.push({ label: 'Reject…', icon: 'pi pi-times', command: () => askReject(t) })
    if (can.handOver(t)) items.push({ label: 'Hand over', icon: 'pi pi-arrow-right-arrow-left', command: () => doHandOver(t) })
    if (can.cancel(t)) items.push({ label: 'Cancel ticket', icon: 'pi pi-ban', command: () => doCancel(t) })
    return items
  }

  return { canManage, can, isOwn, approveTarget, approveOpen, rejectTarget, rejectOpen, rejectCancel, askApprove, askReject, doHandOver, doCancel, menuItems }
}
