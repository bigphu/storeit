// Lưu tên, mô tả của loại (trang chi tiết, ngăn kéo, ô sửa tại chỗ) và archive/restore, có
// Undo. PATCH cần đủ tên và mô tả cùng version
import type { AssetType } from '@/lib/api/types'
import { runAction } from '@/lib/actions'
import type { DraftValue } from '@/lib/detailDraft'
import { useArchiveAssetType, useRestoreAssetType, useUpdateAssetType } from './api'

type TypeRef = Pick<AssetType, 'id' | 'name' | 'description' | 'version'>

export function useTypeOverviewSave() {
  const update = useUpdateAssetType()
  return (t: TypeRef, changes: Record<string, DraftValue>) => {
    const saved = { name: t.name, description: t.description }
    const next = { name: String(changes.name ?? t.name), description: String(changes.description ?? t.description) }
    return runAction({
      run: () => update.mutateAsync({ id: t.id, version: t.version, ...next }),
      done: `${next.name} saved.`,
      failed: `Couldn't save ${t.name}.`,
      undo: (res) => update.mutateAsync({ id: t.id, version: res.version, ...saved }),
      undone: `Changes to ${t.name} undone.`,
      undoFailed: `Couldn't undo the changes to ${t.name}. The saved version stays.`,
    })
  }
}

export function useTypeArchive() {
  const archive = useArchiveAssetType()
  const restore = useRestoreAssetType()
  return (t: Pick<AssetType, 'id' | 'name' | 'archived_at'>) =>
    t.archived_at
      ? runAction({
          run: () => restore.mutateAsync(t.id),
          done: `${t.name} restored.`,
          failed: `Couldn't restore ${t.name}.`,
          undo: () => archive.mutateAsync(t.id),
          undone: `${t.name} archived again.`,
          undoFailed: `Couldn't archive ${t.name} again. It stays available.`,
        })
      : runAction({
          run: () => archive.mutateAsync(t.id),
          done: `${t.name} archived.`,
          failed: `Couldn't archive ${t.name}.`,
          undo: () => restore.mutateAsync(t.id),
          undone: `${t.name} restored.`,
          undoFailed: `Couldn't restore ${t.name}. It is still archived.`,
        })
}
