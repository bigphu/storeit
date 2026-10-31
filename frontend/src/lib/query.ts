import { MutationCache, QueryCache, QueryClient } from '@tanstack/vue-query'
import { ApiError, describeError } from '@/lib/errors'
import { notify } from '@/lib/notify'
import { NoQueryId } from '@/lib/queryKeyId'

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
      // lỗi 4xx không thử lại: kết quả sẽ y như cũ; query không có id thì không có gì để tải
      retry: (count, err) => !(err instanceof ApiError && err.status < 500) && !(err instanceof NoQueryId) && count < 2,
    },
  },
  queryCache: new QueryCache({
    onError: (err, query) => {
      if (err instanceof NoQueryId) return
      if (query.meta?.toast !== false) notify.error(describeError(err))
    },
  }),
  mutationCache: new MutationCache({
    onError: (err, _vars, _ctx, mutation) => {
      if (mutation.options.meta?.toast !== false) notify.error(describeError(err))
    },
  }),
})
