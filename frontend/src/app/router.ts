import { createRouter, createWebHistory, loadRouteLocation } from 'vue-router'
import { useSession } from '@/lib/auth/session'
import { prefetchRoute } from './prefetch'
import { routes } from './routes'

export const router = createRouter({ history: createWebHistory(), routes })

router.beforeEach(async (to) => {
  const session = useSession()
  // Tải song song thay vì nối đuôi: code của trang (route lazy) bắt đầu tải cùng lúc kiểm tra
  // phiên; dữ liệu của trang bắt đầu ngay khi phiên sẵn sàng, trong lúc code còn đang tải.
  // Lỗi tải code để router tự báo khi chuyển trang thật.
  loadRouteLocation(to).catch(() => {})
  await session.start()
  prefetchRoute(to)
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
