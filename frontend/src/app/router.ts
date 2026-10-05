import { createRouter, createWebHistory } from 'vue-router'
import { useSession } from '@/lib/auth/session'
import { routes } from './routes'

export const router = createRouter({ history: createWebHistory(), routes })

router.beforeEach(async (to) => {
  const session = useSession()
  await session.start()
  const isPublic = to.matched.some((r) => r.meta.public)
  if (isPublic) {
    // đã đăng nhập mà mở trang đăng nhập: về trang định tới
    return to.name === 'login' && session.signedIn ? redirectTarget(to.query.redirect) : true
  }
  if (!session.signedIn) return { name: 'login', query: { redirect: to.fullPath } }
  return true
})

// redirectTarget: chỉ nhận đường dẫn nội bộ ("/..."), không chuyển ra trang ngoài
export function redirectTarget(raw: unknown): string {
  return typeof raw === 'string' && raw.startsWith('/') && !raw.startsWith('//') ? raw : '/'
}
