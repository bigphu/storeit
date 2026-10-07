// Đổi tên, lưu bố cục, chia sẻ, xoá profile export; mọi thao tác có Undo. Dùng chung cho
// trang profile, danh sách profile, ngăn kéo sửa nhanh và ô sửa tại chỗ
import { useDeleteExportProfile, useRestoreExportProfile, useUpdateExportProfile } from '@/features/assets/export/api'
import type { ExportLayout, ExportProfile } from '@/lib/api/types'
import { runAction } from '@/lib/actions'
import { confirmAction } from '@/lib/confirm'
import type { DraftValue } from '@/lib/detailDraft'

type ProfileRef = Pick<ExportProfile, 'id' | 'name' | 'version' | 'shared'>

// Trường "visibility" của Overview / ngăn kéo: 'shared' hay 'private'
export const visibilityOf = (p: Pick<ExportProfile, 'shared'>) => (p.shared ? 'shared' : 'private')
export const VISIBILITY_OPTIONS = [
  { label: 'Private', value: 'private' },
  { label: 'Shared', value: 'shared' },
]

// Bản nháp → giá trị gửi đi: tên, chia sẻ, và có đổi chia sẻ không (đổi thì phải hỏi trước)
export function profileChanges(p: Pick<ExportProfile, 'name' | 'shared'>, changes: Record<string, DraftValue>) {
  const name = String(changes.name ?? p.name)
  const shared = changes.visibility === undefined ? p.shared : changes.visibility === 'shared'
  return { name, shared, renamed: name !== p.name, visibility: shared !== p.shared }
}

// Đổi ai thấy profile: hỏi trước rồi mới làm (Undo vẫn có sau khi làm)
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
          icon: 'file',
        },
  )
}

// Lưu Overview (tên, chia sẻ) trong một lần; đổi chia sẻ thì hỏi trước, huỷ thì không lưu gì
export function useProfileOverviewSave() {
  const update = useUpdateExportProfile(false)
  return async (p: ProfileRef, changes: Record<string, DraftValue>) => {
    const c = profileChanges(p, changes)
    if (c.visibility && !(await confirmVisibility(p, c.shared))) return false
    const shareText = c.shared ? 'shared with everyone who can export' : 'private'
    const done = c.renamed && c.visibility ? `${c.name} saved and ${shareText}.` : c.renamed ? `${p.name} renamed to ${c.name}.` : `${p.name} is ${shareText}.`
    return runAction({
      run: () => update.mutateAsync({ id: p.id, version: p.version, name: c.name, shared: c.shared }),
      done,
      failed: `Couldn't save ${p.name}.`,
      undo: (next) => update.mutateAsync({ id: p.id, version: next.version, name: p.name, shared: p.shared }),
      undone: `${p.name} put back as it was.`,
      undoFailed: `Couldn't put ${p.name} back. The saved version stays.`,
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
