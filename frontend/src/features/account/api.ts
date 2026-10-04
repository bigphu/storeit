import { useMutation } from '@tanstack/vue-query'
import { identityApi } from '@/lib/api/client'
import { useSession } from '@/lib/auth/session'
import { unwrap } from '@/lib/errors'

// useUpdateMe: tự đổi tên hiển thị (PATCH /me); phiên nhận ngay tài khoản mới
export function useUpdateMe() {
  const session = useSession()
  return useMutation({
    mutationFn: (body: { name: string; version: number }) => unwrap(identityApi.PATCH('/me', { body })),
    meta: { toast: false },
    onSuccess: (me) => {
      session.me = me
    },
  })
}
