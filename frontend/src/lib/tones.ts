// Màu tag theo nghĩa (spec "Tags"): tốt/dùng được brand, đang dùng info, cần chú ý warn,
// ngừng dùng danger, không hoạt động neutral. Tên là severity của PrimeVue Tag
import type { Account, StatusKind } from '@/lib/api/types'

export type Tone = 'success' | 'info' | 'warn' | 'danger' | 'secondary'

export function statusKindTone(k: StatusKind): Tone {
  return ({ available: 'success', in_use: 'info', unavailable: 'warn', retired: 'danger' } as const)[k]
}

export function accountStatusTone(s: Account['status']): Tone {
  return s === 'active' ? 'success' : s === 'invited' ? 'info' : 'danger'
}
