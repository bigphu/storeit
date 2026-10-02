// Mã quyền của backend (identity/domain, inventory/domain permissions.go)
export const Perm = {
  AccountRead: 'identity.account.read',
  AccountManage: 'identity.account.manage',
  RoleRead: 'identity.role.read',
  RoleManage: 'identity.role.manage',
  AssetRead: 'inventory.asset.read',
  AssetManage: 'inventory.asset.manage',
  TypeManage: 'inventory.type.manage',
  StatusManage: 'inventory.status.manage',
} as const

export type PermCode = (typeof Perm)[keyof typeof Perm]
