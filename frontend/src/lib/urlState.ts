import { computed } from 'vue'
import { type LocationQuery, type LocationQueryRaw, useRoute, useRouter } from 'vue-router'

// useUrlState: state của danh sách (tìm kiếm, lọc, sắp, trang) nằm trên URL, nên
// tải lại hay gửi link vẫn giữ nguyên
export function useUrlState<T>(parse: (q: LocationQuery) => T, serialize: (s: T) => LocationQueryRaw) {
  const route = useRoute()
  const router = useRouter()
  const state = computed(() => parse(route.query))

  function update(patch: Partial<T>) {
    router.replace({ query: serialize({ ...state.value, ...patch }) })
  }

  return { state, update }
}

export function queryString(v: unknown): string | undefined {
  const s = Array.isArray(v) ? v[0] : v
  return typeof s === 'string' && s !== '' ? s : undefined
}

export function queryInt(v: unknown, fallback: number): number {
  const n = Number(queryString(v))
  return Number.isInteger(n) && n > 0 ? n : fallback
}
