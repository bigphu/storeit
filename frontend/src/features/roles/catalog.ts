// Danh mục quyền cho giao diện: nhãn ngắn, và ma trận "khu vực × Xem / Quản lý" của
// trang vai trò. Mã quyền là của backend (lib/auth/permissions.ts).
import { Perm, type PermCode } from '@/lib/auth/permissions'

export const PERM_LABEL: Record<PermCode, string> = {
  [Perm.AccountRead]: 'View accounts',
  [Perm.AccountManage]: 'Manage accounts',
  [Perm.RoleRead]: 'View roles',
  [Perm.RoleManage]: 'Manage roles',
  [Perm.AssetRead]: 'View assets, types and statuses',
  [Perm.AssetManage]: 'Manage assets',
  [Perm.TypeManage]: 'Manage asset types',
  [Perm.StatusManage]: 'Manage statuses',
}

// Role hệ thống có ID cố định (backend identity/domain/permissions.go)
export const ADMINISTRATOR_ROLE_ID = '00000000-0000-7000-8000-000000000001'
export const EMPLOYEE_ROLE_ID = '00000000-0000-7000-8000-000000000004'

export const MODULES = ['Identity', 'Inventory'] as const
export type Module = (typeof MODULES)[number]

export const moduleOf = (code: string): Module => (code.startsWith('identity.') ? 'Identity' : 'Inventory')

// Một khu vực: quyền xem và quyền quản lý. Loại tài sản và status không có quyền xem
// riêng: xem tài sản là xem được cả hai (viaView).
export interface Area {
  module: Module
  label: string
  hint: string
  view?: PermCode
  viaView?: PermCode
  manage: PermCode
}

export const AREAS: Area[] = [
  { module: 'Identity', label: 'Accounts', hint: 'People who can sign in', view: Perm.AccountRead, manage: Perm.AccountManage },
  { module: 'Identity', label: 'Roles', hint: 'Roles and what they allow', view: Perm.RoleRead, manage: Perm.RoleManage },
  { module: 'Inventory', label: 'Assets', hint: 'Assets, plus viewing types and statuses', view: Perm.AssetRead, manage: Perm.AssetManage },
  { module: 'Inventory', label: 'Asset types', hint: 'Types, custom attributes and options', viaView: Perm.AssetRead, manage: Perm.TypeManage },
  { module: 'Inventory', label: 'Statuses', hint: 'Status names, kinds and defaults', viaView: Perm.AssetRead, manage: Perm.StatusManage },
]

export const ALL_PERMS = Object.keys(PERM_LABEL) as PermCode[]
export const label = (code: string) => PERM_LABEL[code as PermCode] ?? code

// toggle: bật/tắt một quyền theo luật của giao diện: bật Quản lý thì bật Xem của khu
// vực đó; tắt Xem thì tắt các Quản lý dựa vào nó. API không đòi luật này.
export function toggle(perms: Iterable<string>, code: string, on: boolean): string[] {
  const set = new Set(perms)
  if (on) {
    set.add(code)
    for (const a of AREAS) if (a.manage === code) set.add((a.view ?? a.viaView)!)
  } else {
    set.delete(code)
    for (const a of AREAS) if ((a.view ?? a.viaView) === code) set.delete(a.manage)
  }
  return ALL_PERMS.filter((p) => set.has(p))
}

export type PermState = 'kept' | 'added' | 'removed'

// effective: quyền có được từ các role, so giữa bản đã lưu và bản đang sửa; nhóm theo module
export function effective(
  roles: { id: string; permissions: string[] }[],
  saved: string[],
  draft: string[],
): { module: Module; items: { code: string; label: string; state: PermState }[] }[] {
  const permsOf = (ids: string[]) => new Set(roles.filter((r) => ids.includes(r.id)).flatMap((r) => r.permissions))
  const before = permsOf(saved)
  const after = permsOf(draft)
  return MODULES.map((module) => ({
    module,
    items: ALL_PERMS.filter((c) => moduleOf(c) === module && (before.has(c) || after.has(c))).map((code) => ({
      code,
      label: label(code),
      state: (after.has(code) ? (before.has(code) ? 'kept' : 'added') : 'removed') as PermState,
    })),
  })).filter((g) => g.items.length)
}
