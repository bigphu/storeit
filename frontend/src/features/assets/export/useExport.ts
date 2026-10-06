// Chạy một lần export kèm thông báo: tên file, cột bị bỏ, hay lỗi (vd quá số dòng)
import { ref } from 'vue'
import type { ExportRequest } from '@/lib/api/types'
import { describeError } from '@/lib/errors'
import { notify } from '@/lib/notify'
import { exportAssets } from './api'

export function useExport() {
  const running = ref(false)
  async function run(body: ExportRequest, fallbackName: string) {
    running.value = true
    try {
      const { name, skipped } = await exportAssets(body, fallbackName)
      notify.success(`Downloaded ${name}.`)
      if (skipped.length) notify.info(`Skipped columns not available for the exported asset types: ${skipped.join(', ')}.`)
    } catch (err) {
      notify.error(describeError(err))
    } finally {
      running.value = false
    }
  }
  return { run, running }
}
