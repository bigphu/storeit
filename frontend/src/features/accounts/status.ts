import type { Account } from '@/lib/api/types'
import { accountStatusTone, type Tone } from '@/lib/tones'

// Màu tag theo trạng thái tài khoản
export function statusSeverity(s: Account['status']): Tone {
  return accountStatusTone(s)
}
