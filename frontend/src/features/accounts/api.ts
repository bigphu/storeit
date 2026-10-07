import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, type MaybeRefOrGetter, toValue } from 'vue'
import { identityApi } from '@/lib/api/client'
import type { Account } from '@/lib/api/types'
import { unwrap } from '@/lib/errors'

export type AccountStatus = Account['status']

export interface AccountListParams {
  q?: string
  status?: AccountStatus
  role_id?: string
  page: number
  page_size: number
}

export const accountKeys = {
  all: ['accounts'] as const,
  list: (p: AccountListParams) => ['accounts', 'list', p] as const,
  one: (id: string) => ['accounts', id] as const,
}

// enabled: false thì không tải (vd tab People của trang vai trò khi chưa mở)
export function useAccounts(params: MaybeRefOrGetter<AccountListParams>, enabled: MaybeRefOrGetter<boolean> = true) {
  return useQuery({
    queryKey: computed(() => accountKeys.list(toValue(params))),
    queryFn: () => unwrap(identityApi.GET('/accounts', { params: { query: toValue(params) } })),
    placeholderData: keepPreviousData,
    enabled: computed(() => toValue(enabled)),
  })
}

export function useAccount(id: MaybeRefOrGetter<string>) {
  return useQuery({
    queryKey: computed(() => accountKeys.one(toValue(id))),
    queryFn: () =>
      unwrap(identityApi.GET('/accounts/{accountID}', { params: { path: { accountID: toValue(id) } } })),
  })
}

// Mọi thay đổi tài khoản làm mới danh sách và chi tiết, và số thành viên của role
function useAccountMutation<V, R>(fn: (v: V) => Promise<R>, toast = true) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: fn,
    meta: { toast },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: accountKeys.all })
      qc.invalidateQueries({ queryKey: ['roles'] })
    },
  })
}

const path = (id: string) => ({ params: { path: { accountID: id } } })

export function useCreateAccount() {
  return useAccountMutation(
    (body: { email: string; name: string; role_ids?: string[] }) => unwrap(identityApi.POST('/accounts', { body })),
    false,
  )
}

export function useUpdateAccount() {
  return useAccountMutation(
    ({ id, name, version }: { id: string; name: string; version: number }) =>
      unwrap(identityApi.PATCH('/accounts/{accountID}', { ...path(id), body: { name, version } })),
    false,
  )
}

// Các mutation dưới đây chạy qua runAction: lỗi do runAction báo (toast: false)
export function useAssignRoles() {
  return useAccountMutation(
    ({ id, roleIds }: { id: string; roleIds: string[] }) =>
      unwrap(identityApi.PUT('/accounts/{accountID}/roles', { ...path(id), body: { role_ids: roleIds } })),
    false,
  )
}

export function useDisableAccount() {
  return useAccountMutation((id: string) => unwrap(identityApi.POST('/accounts/{accountID}/disable', path(id))), false)
}

export function useEnableAccount() {
  return useAccountMutation((id: string) => unwrap(identityApi.POST('/accounts/{accountID}/enable', path(id))), false)
}

export function useResendInvitation() {
  return useAccountMutation((id: string) => unwrap(identityApi.POST('/accounts/{accountID}/invitation', path(id))), false)
}

export function useSendPasswordReset() {
  return useAccountMutation((id: string) => unwrap(identityApi.POST('/accounts/{accountID}/password-reset', path(id))), false)
}

// Đăng xuất account khỏi mọi thiết bị; trả số phiên đã kết thúc
export function useSignOutAccount() {
  return useAccountMutation((id: string) => unwrap(identityApi.POST('/accounts/{accountID}/sign-out', path(id))), false)
}
