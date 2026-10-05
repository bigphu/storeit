import type { Account } from '@/lib/api/types'

// Màu tag theo trạng thái tài khoản
export function statusSeverity(s: Account['status']): 'success' | 'info' | 'secondary' {
  return s === 'active' ? 'success' : s === 'invited' ? 'info' : 'secondary'
}
