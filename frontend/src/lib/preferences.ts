// Tuỳ chọn của người dùng, lưu theo tài khoản trên máy này. Số dòng mỗi trang: bảng
// nào người dùng đã chọn thì giữ số đó, bảng chưa chọn dùng mặc định.
import { defineStore } from 'pinia'
import { computed, type MaybeRefOrGetter, ref, toValue, watch } from 'vue'
import { useSession } from '@/lib/auth/session'
import { readJSON, writeJSON } from '@/lib/storage'

// page_size của API: 1..200
export const PAGE_SIZES = [10, 25, 50, 100]

export interface Prefs {
  defaultPageSize: number
  // khoá bảng ("assets:all", "assets:<typeId>", "accounts") -> số dòng
  tableSizes: Record<string, number>
}

export const defaultPrefs = (): Prefs => ({ defaultPageSize: 50, tableSizes: {} })

export function pageSizeFor(p: Prefs, key: string): number {
  return p.tableSizes[key] ?? p.defaultPageSize
}

export function withTableSize(p: Prefs, key: string, size: number | 'default'): Prefs {
  const tableSizes = { ...p.tableSizes }
  if (size === 'default') delete tableSizes[key]
  else tableSizes[key] = size
  return { ...p, tableSizes }
}

export const usePreferences = defineStore('preferences', () => {
  const session = useSession()
  const prefs = ref<Prefs>(defaultPrefs())
  const storageKey = computed(() => (session.me ? `storeit.prefs.${session.me.account.id}` : null))

  // đăng nhập tài khoản khác thì nạp tuỳ chọn của tài khoản đó
  watch(
    storageKey,
    (key) => {
      prefs.value = key ? { ...defaultPrefs(), ...readJSON<Partial<Prefs>>('local', key, {}) } : defaultPrefs()
    },
    { immediate: true },
  )
  watch(
    prefs,
    (p) => {
      if (storageKey.value) writeJSON('local', storageKey.value, p)
    },
    { deep: true },
  )

  return { prefs }
})

// usePageSize: số dòng của một bảng; set gọi từ sự kiện @page của DataTable
export function usePageSize(key: MaybeRefOrGetter<string>) {
  const store = usePreferences()
  const size = computed(() => pageSizeFor(store.prefs, toValue(key)))
  return {
    size,
    // chỉ ghi khi số dòng thật sự đổi (sự kiện @page cũng bắn khi chỉ đổi trang)
    set(rows: number) {
      if (rows !== size.value) store.prefs = withTableSize(store.prefs, toValue(key), rows)
    },
  }
}
