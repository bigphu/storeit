// Lưu tên, mô tả role (role hệ thống giữ tên) và xoá role, có Undo; dùng chung cho trang
// role, ngăn kéo sửa nhanh và ô sửa tại chỗ trên thẻ
import { runAction } from '@/lib/actions'
import { type DraftValue, previousOf } from '@/lib/detailDraft'
import { useDeleteRole, useRestoreRole, useUpdateRole } from './api'

type Body = { name?: string; description?: string }

export function useRoleOverviewSave() {
  const update = useUpdateRole()
  return (r: { id: string; name: string; description: string }, changes: Record<string, DraftValue>) => {
    const saved = { name: r.name, description: r.description }
    const name = String(changes.name ?? r.name)
    return runAction({
      run: () => update.mutateAsync({ id: r.id, ...(changes as Body) }),
      done: `${name} saved.`,
      failed: `Couldn't save ${r.name}.`,
      undo: () => update.mutateAsync({ id: r.id, ...(previousOf(saved, changes) as Body) }),
      undone: `Changes to ${r.name} undone.`,
      undoFailed: `Couldn't undo the changes to ${r.name}.`,
    })
  }
}

export function useRoleDelete() {
  const remove = useDeleteRole()
  const restore = useRestoreRole()
  return (r: { id: string; name: string }, after?: () => unknown) =>
    runAction({
      run: () => remove.mutateAsync(r.id),
      done: `${r.name} deleted.`,
      failed: `Couldn't delete ${r.name}.`,
      undo: () => restore.mutateAsync(r.id),
      undone: `${r.name} restored.`,
      undoFailed: `Couldn't restore ${r.name}. It stays deleted.`,
      after,
    })
}
