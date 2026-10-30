// Hành động nhanh trên một tài sản, dùng chung cho danh sách và trang tài sản: mở, sửa,
// retire (RetireDialog, có Undo), thay thế (ReplaceDialog, có Undo), restore (chạy ngay, có Undo),
// sửa nhanh từ danh sách
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { runAction, type UndoOptions } from '@/lib/actions'
import type { AssetDetail, ReplacementLink } from '@/lib/api/types'
import { openLocation } from '@/lib/navigation'
import { fetchAsset, useRecordReplacement, useReplaceAsset, useRestoreAsset, useRetireAsset, useUndoReplacement } from './api'
import { type AssetQuickChange, quickAssetBody } from './quickEdit'
import { assetBodyOf, type FormValues } from './values'

export interface ActionAsset {
  id: string
  tag: string
  name: string
  version: number
  retired_at?: string
  // trang tài sản có sẵn; dòng danh sách không có thì đọc lúc restore
  retired_reason?: string
  replaced_by?: ReplacementLink | null
  // hộp Replace: ngày thay không trước ngày mua
  purchase_date?: string
}

export function useAssetActions() {
  const router = useRouter()
  const restoreAsset = useRestoreAsset()
  const retireAsset = useRetireAsset()
  const replaceAsset = useReplaceAsset()
  const recordReplacement = useRecordReplacement()
  const undoReplacementMut = useUndoReplacement()

  // retireTarget + retireOpen gắn vào <RetireDialog>
  const retireTarget = ref<ActionAsset | null>(null)
  const retireOpen = ref(false)
  // replaceTarget + replaceOpen gắn vào <ReplaceDialog>
  const replaceTarget = ref<{ asset: ActionAsset; typeId: string } | null>(null)
  const replaceOpen = ref(false)

  return {
    retireTarget,
    retireOpen,
    replaceTarget,
    replaceOpen,
    open(a: ActionAsset, e?: MouseEvent, newTab?: boolean) {
      openLocation(router, `/assets/${a.id}`, e, newTab)
    },
    edit(a: ActionAsset, e?: MouseEvent) {
      openLocation(router, `/assets/${a.id}/edit`, e)
    },
    askRetire(a: ActionAsset) {
      retireTarget.value = a
      retireOpen.value = true
    },
    askReplace(a: ActionAsset, typeId: string) {
      replaceTarget.value = { asset: a, typeId }
      replaceOpen.value = true
    },
    // Sửa nhanh từ danh sách: đọc bản mới nhất, chỉ đổi phần vừa sửa (attrPatch: thuộc tính
    // vừa sửa, các thuộc tính khác giữ nguyên), PUT với version; Undo đặt lại
    quickSave(row: { id: string; tag: string }, change: AssetQuickChange, what: string, attrPatch?: FormValues) {
      return runAction({
        run: async () => {
          const a = await fetchAsset(row.id)
          const saved = await replaceAsset.mutateAsync({ id: a.id, version: a.version, ...quickAssetBody(a, change, attrPatch) })
          return { before: assetBodyOf(a), saved }
        },
        done: `${row.tag} ${what}.`,
        failed: `Couldn't save ${row.tag}.`,
        undo: ({ before, saved }) => replaceAsset.mutateAsync({ id: row.id, version: saved.version, ...before }),
        undone: `${row.tag} changed back.`,
        undoFailed: `Couldn't change ${row.tag} back. Someone may have changed it since.`,
      })
    },
    // replacementUndo: thông báo sau khi thay (hộp Replace tự gọi API để hiện lỗi trong form);
    // Undo hoàn tác thay thế, tài sản cũ được khôi phục
    replacementUndo(old: ActionAsset, newTag: string): UndoOptions<AssetDetail> {
      return {
        done: `${old.tag} replaced by ${newTag}.`,
        undo: (saved) => undoReplacementMut.mutateAsync({ id: old.id, version: saved.version }),
        undone: `${old.tag} restored.`,
        undoFailed: `Couldn't undo the replacement. Someone may have changed ${old.tag} since.`,
      }
    },
    // undoReplacement: hoàn tác thay thế từ trang tài sản; Undo ghi lại đúng lần thay cũ
    undoReplacement(a: AssetDetail) {
      const by = a.replaced_by
      return runAction({
        run: () => undoReplacementMut.mutateAsync({ id: a.id, version: a.version }),
        done: `Replacement undone. ${a.tag} restored.`,
        failed: `Couldn't undo the replacement of ${a.tag}.`,
        undo: by
          ? (restored) =>
              recordReplacement.mutateAsync({ id: a.id, replacementId: by.id, replacedOn: by.replaced_on, reason: by.reason, version: restored.version })
          : undefined,
        undone: by ? `${a.tag} replaced by ${by.tag} again.` : undefined,
        undoFailed: `Couldn't replace ${a.tag} again. It stays restored.`,
      })
    },
    // restore: tài sản đã bị thay thì restore là hoàn tác thay thế (server bỏ liên kết), nên
    // Undo ghi lại đúng lần thay đó thay vì chỉ retire lại
    restore(a: ActionAsset) {
      return runAction({
        run: async () => {
          // giữ lý do và liên kết thay thế để Undo đặt lại đúng như cũ
          const known = a.retired_reason !== undefined && a.replaced_by !== undefined
          const d = known ? a : await fetchAsset(a.id)
          const restored = await restoreAsset.mutateAsync({ id: a.id, version: a.version })
          return { restored, reason: d.retired_reason ?? '', by: d.replaced_by ?? null }
        },
        done: ({ by }) => (by ? `Replacement undone. ${a.tag} restored.` : `${a.tag} restored.`),
        failed: `Couldn't restore ${a.tag}.`,
        undo: ({ restored, reason, by }) =>
          by
            ? recordReplacement.mutateAsync({ id: a.id, replacementId: by.id, replacedOn: by.replaced_on, reason: by.reason, version: restored.version })
            : retireAsset.mutateAsync({ id: a.id, reason, version: restored.version }),
        undone: ({ by }) => (by ? `${a.tag} replaced by ${by.tag} again.` : `${a.tag} retired again.`),
        undoFailed: `Couldn't retire ${a.tag} again. It stays restored.`,
      })
    },
  }
}
