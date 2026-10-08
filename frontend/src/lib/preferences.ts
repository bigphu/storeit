// Tuỳ chọn của người dùng, lưu theo tài khoản trên máy này. Số dòng mỗi trang: bảng
// nào người dùng đã chọn thì giữ số đó, bảng chưa chọn dùng mặc định.
import { defineStore } from 'pinia'
import { computed, type MaybeRefOrGetter, ref, toValue, watch } from 'vue'
import { useSession } from '@/lib/auth/session'
import { readJSON, writeJSON } from '@/lib/storage'

// page_size của API: 1..200
export const PAGE_SIZES = [10, 25, 50, 100]

export type Theme = 'system' | 'light' | 'dark'
export type Density = 'comfortable' | 'compact'

export interface Prefs {
  theme: Theme
  density: Density
  defaultPageSize: number
  // khoá bảng ("assets:all", "assets:<typeId>", "accounts") -> số dòng
  tableSizes: Record<string, number>
  // loại tài sản mở gần đây, mới nhất trước (bộ chọn loại)
  recentTypes: string[]
  // đăng nhập lại thì mở lại mọi tab; tắt thì chỉ tab ghim
  reopenTabs: boolean
}

export const defaultPrefs = (): Prefs => ({
  theme: 'system',
  density: 'comfortable',
  // 25: bảng dựng nhanh gấp đôi so với 50 lúc mở trang; ai cần nhiều hơn chọn ở dưới bảng
  defaultPageSize: 25,
  tableSizes: {},
  recentTypes: [],
  reopenTabs: true,
})

export function pushRecent(list: string[], id: string, max = 5): string[] {
  return [id, ...list.filter((x) => x !== id)].slice(0, max)
}

export function isDark(theme: Theme, systemDark: boolean): boolean {
  return theme === 'dark' || (theme === 'system' && systemDark)
}

// tableLabel: tên bảng cho trang Preferences
export function tableLabel(key: string, typeName: (id: string) => string | undefined): string {
  if (key === 'assets:all') return 'All assets'
  if (key.startsWith('assets:')) return typeName(key.slice(7)) ?? 'A removed asset type'
  if (key === 'accounts') return 'Accounts'
  return key
}

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

  // Theme và mật độ áp lên <html>: PrimeVue bật màu tối theo class app-dark
  // (darkModeSelector trong main.ts); "system" theo cài đặt của máy
  const media = typeof window !== 'undefined' ? window.matchMedia('(prefers-color-scheme: dark)') : null
  const systemDark = ref(media?.matches ?? false)
  media?.addEventListener('change', (e) => (systemDark.value = e.matches))
  watch(
    [() => prefs.value.theme, () => prefs.value.density, systemDark],
    ([theme, density, sys]) => {
      const root = document.documentElement
      root.classList.toggle('app-dark', isDark(theme, sys))
      root.classList.toggle('density-compact', density === 'compact')
    },
    { immediate: true },
  )

  function touchType(id: string) {
    if (prefs.value.recentTypes[0] !== id) prefs.value.recentTypes = pushRecent(prefs.value.recentTypes, id)
  }

  return { prefs, touchType }
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
