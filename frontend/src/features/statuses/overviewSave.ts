// Lưu tên status (trang chi tiết, ngăn kéo, ô sửa tại chỗ); archive, restore, đặt mặc định.
// Mọi thao tác có Undo
import type { Status } from '@/lib/api/types'
import { runAction } from '@/lib/actions'
import type { DraftValue } from '@/lib/detailDraft'
import { useArchiveStatus, useRestoreStatus, useUpdateStatus } from './api'
import { KIND_INFO } from './lanes'

export function useStatusOverviewSave() {
  const update = useUpdateStatus()
  return (s: Pick<Status, 'id' | 'name'>, changes: Record<string, DraftValue>) => {
    const name = String(changes.name ?? s.name)
    return runAction({
      run: () => update.mutateAsync({ id: s.id, name }),
      done: `${name} saved.`,
      failed: `Couldn't save ${s.name}.`,
      undo: () => update.mutateAsync({ id: s.id, name: s.name }),
      undone: `${s.name} renamed back.`,
      undoFailed: `Couldn't rename it back. It stays ${name}.`,
    })
  }
}

export function useStatusLifecycle() {
  const archive = useArchiveStatus()
  const restore = useRestoreStatus()
  const update = useUpdateStatus()
  return {
    archive: (s: Pick<Status, 'id' | 'name'>) =>
      runAction({
        run: () => archive.mutateAsync(s.id),
        done: `${s.name} archived.`,
        failed: `Couldn't archive ${s.name}.`,
        undo: () => restore.mutateAsync(s.id),
        undone: `${s.name} restored.`,
        undoFailed: `Couldn't restore ${s.name}. It is still archived.`,
      }),
    restore: (s: Pick<Status, 'id' | 'name'>) =>
      runAction({
        run: () => restore.mutateAsync(s.id),
        done: `${s.name} restored.`,
        failed: `Couldn't restore ${s.name}.`,
        undo: () => archive.mutateAsync(s.id),
        undone: `${s.name} archived again.`,
        undoFailed: `Couldn't archive ${s.name} again. It stays available.`,
      }),
    makeDefault(s: Status, all: Status[]) {
      const prev = all.find((x) => x.kind === s.kind && x.is_default && !x.archived_at)
      const kind = KIND_INFO[s.kind].label.toLowerCase()
      return runAction({
        run: () => update.mutateAsync({ id: s.id, make_default: true }),
        done: `${s.name} is now the default ${kind} status.`,
        failed: `Couldn't make ${s.name} the default.`,
        undo: prev ? () => update.mutateAsync({ id: prev.id, make_default: true }) : undefined,
        undone: prev && `${prev.name} is the default ${kind} status again.`,
        undoFailed: `Couldn't change the default back. ${s.name} is still the default.`,
      })
    },
  }
}
