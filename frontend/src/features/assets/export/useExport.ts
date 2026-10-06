// Chạy một lần export kèm thông báo: tên file, cột bị bỏ, hay lỗi (vd quá số dòng)
import { ref } from 'vue'
import type { ExportRequest } from '@/lib/api/types'
import { describeError } from '@/lib/errors'
import { notify } from '@/lib/notify'
import { exportAssets } from './api'

// Thông tin cho thông báo sau khi tải: số dòng (phạm vi đã biết), mở "What’s inside"
export interface ExportInfo {
  rows?: number
  inside?: () => void
}

export function useExport() {
  const running = ref(false)
  // trả true khi tải xong, false khi lỗi (đã báo)
  async function run(body: ExportRequest, fallbackName: string, info: ExportInfo = {}): Promise<boolean> {
    // đang chạy thì bỏ qua lần bấm thêm, tránh tải hai file cùng lúc
    if (running.value) return false
    running.value = true
    try {
      const { name, skipped } = await exportAssets(body, fallbackName)
      const rows = info.rows === undefined ? '' : ` with ${info.rows} ${info.rows === 1 ? 'row' : 'rows'}`
      notify.success(`Downloaded ${name}${rows}.`, info.inside ? { label: 'What’s inside', run: info.inside } : undefined)
      if (skipped.length) notify.info(`Skipped columns not available for the exported asset types: ${skipped.join(', ')}.`)
      return true
    } catch (err) {
      notify.error(describeError(err))
      return false
    } finally {
      running.value = false
    }
  }
  return { run, running }
}
