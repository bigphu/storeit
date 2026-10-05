// Danh sách người dùng vừa xem: để trang tài sản quay về đúng danh sách đó (cùng bộ
// lọc, trang) và bước tới/lui qua kết quả của nó. Lưu theo tab trình duyệt.
import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { readJSON, writeJSON } from '@/lib/storage'
import type { AssetListState, TypeViews } from './listQuery'

export interface ListContext {
  state: AssetListState
  pageSize: number
  // id các dòng của trang đang xem, theo thứ tự
  ids: string[]
  total: number
}

// position: vị trí (từ 0) của tài sản trong toàn bộ kết quả
export function position(ctx: ListContext, id: string): { index: number; total: number } | null {
  const i = ctx.ids.indexOf(id)
  if (i < 0) return null
  return { index: (ctx.state.page - 1) * ctx.pageSize + i, total: ctx.total }
}

// stepFrom: tài sản kế tiếp (dir 1) hay trước (-1). Sang trang bên cạnh thì trả trang
// cần tải và lấy dòng đầu hay cuối của trang đó
export type Step = { id: string } | { page: number; pick: 'first' | 'last' } | null

export function stepFrom(ctx: ListContext, id: string, dir: 1 | -1): Step {
  const p = position(ctx, id)
  if (!p) return null
  const target = p.index + dir
  if (target < 0 || target >= ctx.total) return null
  const i = ctx.ids.indexOf(id) + dir
  if (i >= 0 && i < ctx.ids.length) return { id: ctx.ids[i] }
  return { page: ctx.state.page + dir, pick: dir > 0 ? 'first' : 'last' }
}

const CTX_KEY = 'storeit.assets.listContexts'
const VIEWS_KEY = 'storeit.assets.typeViews'

// Mỗi tab trong app có danh sách "vừa xem" riêng (khoá: id tab); view đã nhớ của
// từng loại dùng chung mọi tab
export const useListContext = defineStore('assetListContext', () => {
  const ctxs = ref<Record<string, ListContext>>(readJSON<Record<string, ListContext>>('session', CTX_KEY, {}))
  const views = ref<TypeViews>(readJSON<TypeViews>('session', VIEWS_KEY, {}))
  watch(ctxs, (v) => writeJSON('session', CTX_KEY, v), { deep: true })
  watch(views, (v) => writeJSON('session', VIEWS_KEY, v), { deep: true })

  const ctxFor = (tabId: string | null) => (tabId ? (ctxs.value[tabId] ?? null) : null)
  function setCtx(tabId: string | null, c: ListContext) {
    if (tabId) ctxs.value = { ...ctxs.value, [tabId]: c }
  }
  return { ctxs, ctxFor, setCtx, views }
})
