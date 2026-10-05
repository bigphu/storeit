import { MutationCache, QueryCache, QueryClient } from '@tanstack/vue-query'
import { ApiError, describeError } from '@/lib/errors'
import { notify } from '@/lib/notify'

declare module '@tanstack/vue-query' {
  interface Register {
    // toast: false khi form tự hiện lỗi (theo field) thay vì toast
    mutationMeta: { toast?: boolean }
    queryMeta: { toast?: boolean }
  }
}

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
      // lỗi 4xx không thử lại: kết quả sẽ y như cũ
      retry: (count, err) => !(err instanceof ApiError && err.status < 500) && count < 2,
    },
  },
  queryCache: new QueryCache({
    onError: (err, query) => {
      if (query.meta?.toast !== false) notify.error(describeError(err))
    },
  }),
  mutationCache: new MutationCache({
    onError: (err, _vars, _ctx, mutation) => {
      if (mutation.options.meta?.toast !== false) notify.error(describeError(err))
    },
  }),
})
