// Đổi tên, lưu bố cục, chia sẻ, xoá profile export; mọi thao tác có Undo. Dùng chung cho
// trang profile, danh sách profile, ngăn kéo sửa nhanh và ô sửa tại chỗ
import { useDeleteExportProfile, useRestoreExportProfile, useUpdateExportProfile } from '@/features/assets/export/api'
import type { ExportLayout, ExportProfile } from '@/lib/api/types'
import { runAction } from '@/lib/actions'
import { confirmAction } from '@/lib/confirm'
import type { DraftValue } from '@/lib/detailDraft'

type ProfileRef = Pick<ExportProfile, 'id' | 'name' | 'version' | 'shared'>

// Đổi ai thấy profile: hỏi trước rồi mới làm (Undo vẫn có sau khi làm). Làm riêng tư lấy
// profile khỏi tay người khác nên là cảnh báo (cam)
function confirmVisibility(p: Pick<ExportProfile, 'name'>, sharing: boolean) {
  return confirmAction(
    sharing
      ? {
          title: `Share ${p.name}?`,
          body: 'Everyone who can export sees this profile and can export with it. Only its owner or a profile manager can change it.',
          action: 'Share profile',
          danger: false,
          icon: 'file',
        }
      : {
          title: `Make ${p.name} private?`,
          body: 'It leaves everyone else’s list of profiles. Only its owner can export with it.',
          action: 'Make private',
          danger: false,
          warn: true,
          icon: 'alert',
        },
  )
}

export function useProfileOverviewSave() {
  const update = useUpdateExportProfile(false)
  return (p: ProfileRef, changes: Record<string, DraftValue>) => {
    const name = String(changes.name ?? p.name)
    return runAction({
      run: () => update.mutateAsync({ id: p.id, version: p.version, name }),
      done: `${p.name} renamed to ${name}.`,
      failed: `Couldn't rename ${p.name}.`,
      undo: (next) => update.mutateAsync({ id: p.id, version: next.version, name: p.name }),
      undone: `Name put back to ${p.name}.`,
      undoFailed: `Couldn't put the name back. It stays ${name}.`,
    })
  }
}

export function useProfileLayoutSave() {
  const update = useUpdateExportProfile(false)
  return (p: Pick<ExportProfile, 'id' | 'name' | 'version' | 'layout'>, layout: ExportLayout) =>
    runAction({
      run: () => update.mutateAsync({ id: p.id, version: p.version, layout }),
      done: `${p.name} saved.`,
      failed: `Couldn't save ${p.name}.`,
      undo: (next) => update.mutateAsync({ id: p.id, version: next.version, layout: p.layout }),
      undone: `${p.name} put back as it was.`,
      undoFailed: `Couldn't put ${p.name} back. The saved version stays.`,
    })
}

export function useProfileShare() {
  const update = useUpdateExportProfile(false)
  return async (p: ProfileRef) => {
    const sharing = !p.shared
    if (!(await confirmVisibility(p, sharing))) return false
    return runAction({
      run: () => update.mutateAsync({ id: p.id, version: p.version, shared: sharing }),
      done: sharing ? `${p.name} is shared with everyone who can export.` : `${p.name} is private again.`,
      failed: `Couldn't change who sees ${p.name}.`,
      undo: (next) => update.mutateAsync({ id: p.id, version: next.version, shared: p.shared }),
      undone: sharing ? `${p.name} is private again.` : `${p.name} is shared again.`,
      undoFailed: `Couldn't change who sees ${p.name} back.`,
    })
  }
}

export function useProfileDelete() {
  const remove = useDeleteExportProfile()
  const restore = useRestoreExportProfile()
  return (p: Pick<ExportProfile, 'id' | 'name'>, after?: () => unknown) =>
    runAction({
      run: () => remove.mutateAsync(p.id),
      done: `${p.name} deleted.`,
      failed: `Couldn't delete ${p.name}.`,
      undo: () => restore.mutateAsync(p.id),
      undone: `${p.name} restored.`,
      undoFailed: `Couldn't restore ${p.name}. It stays deleted.`,
      after,
    })
}
