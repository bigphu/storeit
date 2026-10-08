// Lưu tên tài khoản (PATCH cần version) và role của tài khoản, có Undo; dùng chung cho trang
// tài khoản, ngăn kéo sửa nhanh và ô sửa tại chỗ
import type { Account } from '@/lib/api/types'
import { runAction } from '@/lib/actions'
import type { DraftValue } from '@/lib/detailDraft'
import { useAssignRoles, useUpdateAccount } from './api'

export function useAccountOverviewSave() {
  const update = useUpdateAccount()
  return (a: Pick<Account, 'id' | 'name' | 'version'>, changes: Record<string, DraftValue>) => {
    const name = String(changes.name ?? a.name)
    return runAction({
      run: () => update.mutateAsync({ id: a.id, name, version: a.version }),
      done: `${name} saved.`,
      failed: `Couldn't save ${a.name}.`,
      undo: (res) => update.mutateAsync({ id: a.id, name: a.name, version: res.version }),
      undone: `${a.name} renamed back.`,
      undoFailed: `Couldn't rename it back. It stays ${name}.`,
    })
  }
}

export function useAccountRolesSave() {
  const assign = useAssignRoles()
  return (a: { id: string; name: string }, before: string[], next: string[]) =>
    runAction({
      run: () => assign.mutateAsync({ id: a.id, roleIds: next }),
      done: `Roles of ${a.name} saved.`,
      failed: "Couldn't save the roles.",
      undo: () => assign.mutateAsync({ id: a.id, roleIds: before }),
      undone: 'Roles put back.',
      undoFailed: "Couldn't put the roles back. The new roles stay.",
    })
}
