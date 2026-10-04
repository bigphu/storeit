// State của danh sách tài sản <-> URL <-> tham số GET /assets.
// URL dùng đúng tên tham số API: q, type_id, status_id, status_kind,
// include_retired, attr (lặp lại), sort, page.
import type { LocationQuery, LocationQueryRaw } from 'vue-router'
import type { DataType, StatusKind } from '@/lib/api/types'
import { queryInt, queryString } from '@/lib/urlState'

export interface AttrFilterRow {
  key: string
  op: string
  value: string
}

export interface AssetListState {
  q: string
  typeId?: string
  statusId?: string
  statusKind?: StatusKind
  includeRetired: boolean
  filters: AttrFilterRow[]
  // giá trị sort của API: "name", "-updated_at", "attributes.ram_gb"...
  sort?: string
  page: number
}

const statusKinds: StatusKind[] = ['available', 'in_use', 'unavailable', 'retired']

// Toán tử theo kiểu dữ liệu, khớp backend (domain/attr_query.go)
const ops: Record<DataType, string[]> = {
  text: ['eq', 'contains'],
  number: ['eq', 'gt', 'gte', 'lt', 'lte'],
  date: ['eq', 'gt', 'gte', 'lt', 'lte'],
  boolean: ['eq'],
  select: ['eq', 'in'],
}

export function operatorsFor(t: DataType): string[] {
  return ops[t]
}

// "<key>:<op>:<value>"; giá trị có thể chứa ':'
function parseFilter(raw: string): AttrFilterRow | undefined {
  const m = /^([a-z][a-z0-9_]{0,31}):([a-z]+):(.+)$/.exec(raw)
  return m ? { key: m[1], op: m[2], value: m[3] } : undefined
}

export function parseAssetQuery(q: LocationQuery): AssetListState {
  const attr = q.attr === undefined ? [] : Array.isArray(q.attr) ? q.attr : [q.attr]
  const kind = queryString(q.status_kind)
  return {
    q: queryString(q.q) ?? '',
    typeId: queryString(q.type_id),
    statusId: queryString(q.status_id),
    statusKind: statusKinds.includes(kind as StatusKind) ? (kind as StatusKind) : undefined,
    includeRetired: queryString(q.include_retired) === 'true',
    filters: attr.flatMap((a) => (typeof a === 'string' ? (parseFilter(a) ?? []) : [])),
    sort: queryString(q.sort),
    page: queryInt(q.page, 1),
  }
}

const filterString = (f: AttrFilterRow) => `${f.key}:${f.op}:${f.value}`

export function serializeAssetQuery(s: AssetListState): LocationQueryRaw {
  const out: LocationQueryRaw = {}
  if (s.q) out.q = s.q
  if (s.typeId) out.type_id = s.typeId
  if (s.statusId) out.status_id = s.statusId
  if (s.statusKind) out.status_kind = s.statusKind
  if (s.includeRetired) out.include_retired = 'true'
  if (s.filters.length) out.attr = s.filters.map(filterString)
  if (s.sort) out.sort = s.sort
  if (s.page > 1) out.page = String(s.page)
  return out
}

const isAttrSort = (sort?: string) => !!sort && sort.replace(/^-/, '').startsWith('attributes.')

// toApiParams: lọc và sắp theo thuộc tính chỉ gửi khi có loại (backend bắt buộc)
export function toApiParams(s: AssetListState, pageSize: number) {
  const filters = s.typeId ? s.filters.filter((f) => f.value !== '').map(filterString) : []
  return {
    q: s.q || undefined,
    type_id: s.typeId,
    status_id: s.statusId,
    status_kind: s.statusKind,
    include_retired: s.includeRetired,
    attr: filters.length ? filters : undefined,
    sort: s.typeId || !isAttrSort(s.sort) ? s.sort : undefined,
    page: s.page,
    page_size: pageSize,
  }
}

// changeType: thuộc tính thuộc về loại, nên đổi loại thì bỏ lọc và sắp theo thuộc tính
export function changeType(s: AssetListState, typeId: string | undefined): AssetListState {
  return { ...s, typeId, filters: [], sort: isAttrSort(s.sort) ? undefined : s.sort, page: 1 }
}

// Bộ lọc thuộc tính, sắp và trang thuộc về một loại; mỗi loại nhớ của riêng nó
// (khoá '' là danh sách mọi loại). Tìm kiếm, status, "include retired" dùng chung.
export interface TypeView {
  filters: AttrFilterRow[]
  sort?: string
  page: number
}
export type TypeViews = Record<string, TypeView>

// switchType: lưu view của loại đang rời, mở loại mới với view đã nhớ (nếu có)
export function switchType(
  s: AssetListState,
  typeId: string | undefined,
  views: TypeViews,
): { state: AssetListState; views: TypeViews } {
  const next = { ...views, [s.typeId ?? '']: { filters: s.filters, sort: s.sort, page: s.page } }
  const saved = next[typeId ?? '']
  const fresh = changeType(s, typeId)
  return { state: saved ? { ...fresh, filters: saved.filters, sort: saved.sort, page: saved.page } : fresh, views: next }
}

// Đường dẫn của danh sách: loại nằm trên path (/types/:id/assets), phần còn lại ở query
export interface ListLocation {
  path: string
  query: LocationQueryRaw
}

export function listLocation(s: AssetListState): ListLocation {
  if (!s.typeId) return { path: '/assets', query: serializeAssetQuery(s) }
  return { path: `/types/${s.typeId}/assets`, query: serializeAssetQuery({ ...s, typeId: undefined }) }
}

// typeListLocation: mở danh sách của một loại với view đã nhớ (sidebar, breadcrumb)
export function typeListLocation(typeId: string, views: TypeViews): ListLocation {
  const v = views[typeId]
  return listLocation({ q: '', includeRetired: false, typeId, filters: v?.filters ?? [], sort: v?.sort, page: v?.page ?? 1 })
}

// legacyListRedirect: link cũ /assets?type_id=… sang /types/…/assets
export function legacyListRedirect(q: LocationQuery): ListLocation | null {
  const typeId = queryString(q.type_id)
  if (!typeId) return null
  const query: LocationQueryRaw = { ...q }
  delete query.type_id
  return { path: `/types/${typeId}/assets`, query }
}

// fromTableSort: sự kiện sort của DataTable (removable-sort: tăng -> giảm -> bỏ) -> sort của API
export function fromTableSort(field: string | undefined, order: number | null | undefined): string | undefined {
  if (!field || !order) return undefined
  return order < 0 ? `-${field}` : field
}

// toTableSort: sort của API -> sortField/sortOrder của DataTable (để hiện mũi tên)
export function toTableSort(sort: string | undefined): { field?: string; order: 1 | -1 | 0 } {
  if (!sort) return { field: undefined, order: 0 }
  return sort.startsWith('-') ? { field: sort.slice(1), order: -1 } : { field: sort, order: 1 }
}
