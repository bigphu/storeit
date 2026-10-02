import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'

declare module 'vue-router' {
  interface RouteMeta {
    // trang công khai (layout đơn giản, không cần đăng nhập)
    public?: boolean
    // quyền cần để xem; thiếu thì AppLayout hiện "No access"
    perm?: string
  }
}

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    component: () => import('./layouts/PublicLayout.vue'),
    meta: { public: true },
    children: [
      { path: 'login', name: 'login', component: () => import('@/features/auth/pages/LoginPage.vue') },
      {
        path: 'forgot-password',
        name: 'forgot-password',
        component: () => import('@/features/auth/pages/ForgotPasswordPage.vue'),
      },
      {
        path: 'accept-invite',
        name: 'accept-invite',
        component: () => import('@/features/auth/pages/SetPasswordPage.vue'),
        props: { mode: 'invite' },
      },
      {
        path: 'reset-password',
        name: 'reset-password',
        component: () => import('@/features/auth/pages/SetPasswordPage.vue'),
        props: { mode: 'reset' },
      },
    ],
  },
  {
    path: '/',
    component: () => import('./layouts/AppLayout.vue'),
    children: [
      { path: '', redirect: '/assets' },
      {
        path: 'assets',
        name: 'assets',
        component: () => import('@/features/assets/pages/AssetsPage.vue'),
        meta: { perm: Perm.AssetRead },
      },
      {
        path: 'assets/new',
        name: 'asset-new',
        component: () => import('@/features/assets/pages/AssetFormPage.vue'),
        meta: { perm: Perm.AssetManage },
      },
      {
        path: 'assets/:id',
        name: 'asset',
        component: () => import('@/features/assets/pages/AssetDetailPage.vue'),
        props: true,
        meta: { perm: Perm.AssetRead },
      },
      {
        path: 'assets/:id/edit',
        name: 'asset-edit',
        component: () => import('@/features/assets/pages/AssetFormPage.vue'),
        props: true,
        meta: { perm: Perm.AssetManage },
      },
      {
        path: 'asset-types',
        name: 'asset-types',
        component: () => import('@/features/asset-types/pages/AssetTypesPage.vue'),
        meta: { perm: Perm.AssetRead },
      },
      {
        path: 'asset-types/:id',
        name: 'asset-type',
        component: () => import('@/features/asset-types/pages/AssetTypeDetailPage.vue'),
        props: true,
        meta: { perm: Perm.AssetRead },
      },
      {
        path: 'statuses',
        name: 'statuses',
        component: () => import('@/features/statuses/pages/StatusesPage.vue'),
        meta: { perm: Perm.AssetRead },
      },
      {
        path: 'accounts',
        name: 'accounts',
        component: () => import('@/features/accounts/pages/AccountsPage.vue'),
        meta: { perm: Perm.AccountRead },
      },
      {
        path: 'accounts/:id',
        name: 'account',
        component: () => import('@/features/accounts/pages/AccountDetailPage.vue'),
        props: true,
        meta: { perm: Perm.AccountRead },
      },
      {
        path: 'roles',
        name: 'roles',
        component: () => import('@/features/roles/pages/RolesPage.vue'),
        meta: { perm: Perm.RoleRead },
      },
      {
        path: 'roles/:id',
        name: 'role',
        component: () => import('@/features/roles/pages/RoleDetailPage.vue'),
        props: true,
        meta: { perm: Perm.RoleRead },
      },
      {
        path: 'account/password',
        name: 'change-password',
        component: () => import('@/features/auth/pages/ChangePasswordPage.vue'),
      },
      { path: ':path(.*)*', name: 'not-found', component: () => import('./pages/NotFoundPage.vue') },
    ],
  },
]

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
