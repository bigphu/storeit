// Hành động nhanh trên một tài sản, dùng chung cho danh sách và trang tài sản:
// mở, sửa, retire (qua RetireDialog), restore (hỏi lại bằng ConfirmDialog)
import { useConfirm } from 'primevue/useconfirm'
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { notify } from '@/lib/notify'
import { openLocation } from '@/lib/navigation'
import { useRestoreAsset } from './api'

export interface ActionAsset {
  id: string
  tag: string
  name: string
  version: number
  retired_at?: string
}

export function useAssetActions() {
  const router = useRouter()
  const confirm = useConfirm()
  const restore = useRestoreAsset()

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
    askRestore(a: ActionAsset) {
      confirm.require({
        message: `Restore ${a.tag}? It goes back to the default available status.`,
        header: 'Restore asset',
        acceptLabel: 'Restore',
        rejectLabel: 'Cancel',
        accept: () =>
          restore
            .mutateAsync({ id: a.id, version: a.version })
            .then(() => notify.success(`${a.tag} restored.`))
            .catch(() => {}),
      })
    },
  }
}
