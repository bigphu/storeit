import { type QueryClient, useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, type MaybeRefOrGetter, toValue } from 'vue'
import { identityApi } from '@/lib/api/client'
import { unwrap } from '@/lib/errors'

export const roleKeys = {
  all: ['roles'] as const,
  one: (id: string) => ['roles', id] as const,
  permissions: ['permissions'] as const,
}

export function useRoles() {
  return useQuery({ queryKey: roleKeys.all, queryFn: () => unwrap(identityApi.GET('/roles')) })
}

export function useRole(id: MaybeRefOrGetter<string>) {
  return useQuery({
    queryKey: computed(() => roleKeys.one(toValue(id))),
    queryFn: () => unwrap(identityApi.GET('/roles/{roleID}', { params: { path: { roleID: toValue(id) } } })),
  })
}

export function usePermissions() {
  return useQuery({
    queryKey: roleKeys.permissions,
    queryFn: () => unwrap(identityApi.GET('/permissions')),
    staleTime: Infinity,
  })
}

// Mọi thay đổi vai trò làm mới danh sách vai trò (và vai trò đó)
function useRoleMutation<V, R>(fn: (v: V) => Promise<R>, toast = true) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: fn,
    meta: { toast },
    onSuccess: () => qc.invalidateQueries({ queryKey: roleKeys.all }),
  })
}

// permissions: bắt đầu từ quyền của một role khác ("Copy …"); không có là role trống
export function useCreateRole() {
  return useRoleMutation(
    (body: { name: string; description?: string; permissions?: string[] }) =>
      unwrap(identityApi.POST('/roles', { body })),
    false,
  )
}

export function useUpdateRole() {
  return useRoleMutation(
    ({ id, ...body }: { id: string; name?: string; description?: string }) =>
      unwrap(identityApi.PATCH('/roles/{roleID}', { params: { path: { roleID: id } }, body })),
    false,
  )
}

export function useSetRolePermissions() {
  return useRoleMutation(
    ({ id, permissions }: { id: string; permissions: string[] }) =>
      unwrap(identityApi.PUT('/roles/{roleID}/permissions', { params: { path: { roleID: id } }, body: { permissions } })),
    false,
  )
}

// Sau khi xoá role: bỏ hẳn query của role đó (trang của nó sắp rời đi; nạp lại là 404 và
// hiện lỗi), chỉ làm mới danh sách
export function forgetDeletedRole(qc: QueryClient, id: string) {
  qc.removeQueries({ queryKey: roleKeys.one(id), exact: true })
  return qc.invalidateQueries({ queryKey: roleKeys.all, exact: true })
}

export function useDeleteRole() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => unwrap(identityApi.DELETE('/roles/{roleID}', { params: { path: { roleID: id } } })),
    meta: { toast: false },
    onSuccess: (_r, id) => forgetDeletedRole(qc, id),
  })
}

// Undo của xoá role (xoá mềm)
export function useRestoreRole() {
  return useRoleMutation(
    (id: string) => unwrap(identityApi.POST('/roles/{roleID}/restore', { params: { path: { roleID: id } } })),
    false,
  )
}
