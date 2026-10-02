import type { RouteRecordRaw } from 'vue-router'
import { Perm } from '@/lib/auth/permissions'

declare module 'vue-router' {
  interface RouteMeta {
    // trang công khai (layout đơn giản, không cần đăng nhập)
    public?: boolean
    // quyền cần để xem; thiếu thì AppLayout hiện "No access"
    perm?: string
  }
}

// publicPage: một trang công khai bọc trong PublicLayout. Mỗi trang một route riêng,
// không dùng chung cha "/": cha có path "/" sẽ khớp chính "/" và hiện layout rỗng.
function publicPage(
  path: string,
  name: string,
  component: RouteComponent,
  props?: Record<string, unknown>,
): RouteRecordRaw {
  return {
    path,
    component: () => import('./layouts/PublicLayout.vue'),
    meta: { public: true },
    children: [{ path: '', name, component, props }],
  }
}

type RouteComponent = () => Promise<unknown>

export const routes: RouteRecordRaw[] = [
  publicPage('/login', 'login', () => import('@/features/auth/pages/LoginPage.vue')),
  publicPage('/forgot-password', 'forgot-password', () => import('@/features/auth/pages/ForgotPasswordPage.vue')),
  publicPage('/accept-invite', 'accept-invite', () => import('@/features/auth/pages/SetPasswordPage.vue'), {
    mode: 'invite',
  }),
  publicPage('/reset-password', 'reset-password', () => import('@/features/auth/pages/SetPasswordPage.vue'), {
    mode: 'reset',
  }),
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
