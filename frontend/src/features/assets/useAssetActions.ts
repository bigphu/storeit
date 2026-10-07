// Hành động nhanh trên một tài sản, dùng chung cho danh sách và trang tài sản: mở, sửa,
// retire (RetireDialog, có Undo), restore (chạy ngay, có Undo), sửa nhanh từ danh sách
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { runAction } from '@/lib/actions'
import { openLocation } from '@/lib/navigation'
import { fetchAsset, useReplaceAsset, useRestoreAsset, useRetireAsset } from './api'
import { type AssetQuickChange, quickAssetBody } from './quickEdit'
import { assetBodyOf } from './values'

export interface ActionAsset {
  id: string
  tag: string
  name: string
  version: number
  retired_at?: string
  // trang tài sản có sẵn; dòng danh sách không có thì đọc lúc restore
  retired_reason?: string
}

export function useAssetActions() {
  const router = useRouter()
  const restoreAsset = useRestoreAsset()
  const retireAsset = useRetireAsset()
  const replaceAsset = useReplaceAsset()

  // retireTarget + retireOpen gắn vào <RetireDialog>
  const retireTarget = ref<ActionAsset | null>(null)
  const retireOpen = ref(false)

  return {
    retireTarget,
    retireOpen,
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
    // Sửa nhanh từ danh sách: đọc bản mới nhất, chỉ đổi phần vừa sửa, PUT với version; Undo đặt lại
    quickSave(row: { id: string; tag: string }, change: AssetQuickChange, what: string) {
      return runAction({
        run: async () => {
          const a = await fetchAsset(row.id)
          const saved = await replaceAsset.mutateAsync({ id: a.id, version: a.version, ...quickAssetBody(a, change) })
          return { before: assetBodyOf(a), saved }
        },
        done: `${row.tag} ${what}.`,
        failed: `Couldn't save ${row.tag}.`,
        undo: ({ before, saved }) => replaceAsset.mutateAsync({ id: row.id, version: saved.version, ...before }),
        undone: `${row.tag} changed back.`,
        undoFailed: `Couldn't change ${row.tag} back. Someone may have changed it since.`,
      })
    },
    restore(a: ActionAsset) {
      return runAction({
        run: async () => {
          // giữ lý do để Undo retire lại đúng như cũ
          const reason = a.retired_reason ?? (await fetchAsset(a.id)).retired_reason ?? ''
          const restored = await restoreAsset.mutateAsync({ id: a.id, version: a.version })
          return { restored, reason }
        },
        done: `${a.tag} restored.`,
        failed: `Couldn't restore ${a.tag}.`,
        undo: ({ restored, reason }) => retireAsset.mutateAsync({ id: a.id, reason, version: restored.version }),
        undone: `${a.tag} retired again.`,
        undoFailed: `Couldn't retire ${a.tag} again. It stays restored.`,
      })
    },
  }
}
