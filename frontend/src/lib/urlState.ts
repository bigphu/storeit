import { computed } from 'vue'
import { type LocationQuery, type LocationQueryRaw, useRouter } from 'vue-router'
import { useTabQuery } from '@/app/tabs/tabPage'

// useUrlState: state của danh sách (tìm kiếm, lọc, sắp, trang) nằm trên URL, nên
// tải lại hay gửi link vẫn giữ nguyên
export function useUrlState<T>(parse: (q: LocationQuery) => T, serialize: (s: T) => LocationQueryRaw) {
  const router = useRouter()
  // trang được giữ sống khi chuyển tab: chỉ theo URL của tab mình
  const query = useTabQuery()
  const state = computed(() => parse(query.value))

  function update(patch: Partial<T>) {
    router.replace({ query: serialize({ ...state.value, ...patch }) })
  }

  return { state, update }
}

export { queryInt, queryString } from './queryParams'
