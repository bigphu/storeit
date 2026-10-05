import { describe, expect, it } from 'vitest'
import { Perm } from '@/lib/auth/permissions'
import { effective, toggle } from './catalog'

describe('toggle', () => {
  it('turning Manage on also turns its View on', () => {
    expect(toggle([], Perm.AccountManage, true)).toEqual([Perm.AccountRead, Perm.AccountManage])
    // loại tài sản không có quyền xem riêng: bật xem tài sản
    expect(toggle([], Perm.TypeManage, true)).toEqual([Perm.AssetRead, Perm.TypeManage])
  })

  it('turning View off clears the Manage that depends on it', () => {
    const all = [Perm.AssetRead, Perm.AssetManage, Perm.TypeManage, Perm.StatusManage]
    expect(toggle(all, Perm.AssetRead, false)).toEqual([])
    expect(toggle([Perm.RoleRead, Perm.RoleManage], Perm.RoleManage, false)).toEqual([Perm.RoleRead])
  })
})

describe('effective', () => {
  const roles = [
    { id: 'emp', permissions: [Perm.AssetRead] },
    { id: 'off', permissions: [Perm.AssetRead, Perm.AssetManage] },
    { id: 'hr', permissions: [Perm.AccountRead] },
  ]

  it('marks permissions added and removed by the draft, grouped by module', () => {
    const groups = effective(roles, ['emp', 'hr'], ['off'])
    expect(groups.map((g) => g.module)).toEqual(['Identity', 'Inventory'])
    expect(groups[0].items).toEqual([{ code: Perm.AccountRead, label: 'View accounts', state: 'removed' }])
    expect(groups[1].items.map((i) => [i.code, i.state])).toEqual([
      [Perm.AssetRead, 'kept'],
      [Perm.AssetManage, 'added'],
    ])
  })

  it('is empty without roles', () => {
    expect(effective(roles, [], [])).toEqual([])
  })
})
