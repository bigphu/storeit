import type { RouteRecordRaw } from 'vue-router'
import { Perm } from '@/lib/auth/permissions'
import { legacyListRedirect } from '@/features/assets/listQuery'

declare module 'vue-router' {
  interface RouteMeta {
    // trang công khai (layout đơn giản, không cần đăng nhập)
    public?: boolean
    // quyền cần để xem; thiếu thì AppLayout hiện "No access"
    perm?: string
    // tiêu đề tab mặc định (trang có dữ liệu thì tự đặt tiêu đề cụ thể)
    title?: string
    // biểu tượng PrimeIcons của tab
    icon?: string
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
        meta: { title: 'All assets', icon: 'pi pi-th-large', perm: Perm.AssetRead },
        // link cũ /assets?type_id=… sang danh sách của loại
        beforeEnter: (to) => legacyListRedirect(to.query) ?? true,
      },
      {
        path: 'assets/new',
        name: 'asset-new',
        component: () => import('@/features/assets/pages/AssetFormPage.vue'),
        meta: { title: 'New asset', icon: 'pi pi-plus', perm: Perm.AssetManage },
      },
      {
        path: 'assets/:id',
        name: 'asset',
        component: () => import('@/features/assets/pages/AssetDetailPage.vue'),
        props: true,
        meta: { title: 'Asset', icon: 'pi pi-box', perm: Perm.AssetRead },
      },
      {
        path: 'assets/:id/edit',
        name: 'asset-edit',
        component: () => import('@/features/assets/pages/AssetFormPage.vue'),
        props: true,
        meta: { title: 'Edit asset', icon: 'pi pi-pencil', perm: Perm.AssetManage },
      },
      // Các trang trong phạm vi một loại: sidebar đổi sang các mục của loại đó
      {
        path: 'types',
        name: 'types',
        component: () => import('@/features/asset-types/pages/AssetTypesPage.vue'),
        meta: { title: 'Asset types', icon: 'pi pi-sitemap', perm: Perm.AssetRead },
      },
      {
        path: 'types/:typeId/assets',
        name: 'type-assets',
        component: () => import('@/features/assets/pages/AssetsPage.vue'),
        props: true,
        meta: { title: 'Assets', icon: 'pi pi-list', perm: Perm.AssetRead },
      },
      {
        path: 'types/:typeId/assets/new',
        name: 'type-asset-new',
        component: () => import('@/features/assets/pages/AssetFormPage.vue'),
        props: true,
        meta: { title: 'New asset', icon: 'pi pi-plus', perm: Perm.AssetManage },
      },
      {
        path: 'types/:typeId/settings',
        name: 'type-settings',
        component: () => import('@/features/asset-types/pages/TypeSettingsPage.vue'),
        props: true,
        meta: { title: 'Type settings', icon: 'pi pi-cog', perm: Perm.AssetRead },
      },
      { path: 'asset-types', redirect: '/types' },
      { path: 'asset-types/:id', redirect: (to) => `/types/${to.params.id}/settings` },
      {
        path: 'statuses',
        name: 'statuses',
        component: () => import('@/features/statuses/pages/StatusesPage.vue'),
        meta: { title: 'Statuses', icon: 'pi pi-circle', perm: Perm.AssetRead },
      },
      {
        path: 'export-profiles',
        name: 'export-profiles',
        component: () => import('@/features/export-profiles/pages/ExportProfilesPage.vue'),
        meta: { title: 'Export profiles', icon: 'pi pi-file-export', perm: Perm.AssetExport },
      },
      {
        path: 'accounts',
        name: 'accounts',
        component: () => import('@/features/accounts/pages/AccountsPage.vue'),
        meta: { title: 'Accounts', icon: 'pi pi-users', perm: Perm.AccountRead },
      },
      {
        path: 'accounts/:id',
        name: 'account',
        component: () => import('@/features/accounts/pages/AccountDetailPage.vue'),
        props: true,
        meta: { title: 'Account', icon: 'pi pi-user', perm: Perm.AccountRead },
      },
      {
        path: 'roles',
        name: 'roles',
        component: () => import('@/features/roles/pages/RolesPage.vue'),
        meta: { title: 'Roles', icon: 'pi pi-shield', perm: Perm.RoleRead },
      },
      {
        path: 'roles/:id',
        name: 'role',
        component: () => import('@/features/roles/pages/RoleDetailPage.vue'),
        props: true,
        meta: { title: 'Role', icon: 'pi pi-shield', perm: Perm.RoleRead },
      },
      // Tài khoản của tôi: hồ sơ, mật khẩu, tuỳ chọn
      { path: 'account', redirect: '/account/profile' },
      {
        path: 'account/:section(profile|password|preferences)',
        name: 'account-settings',
        component: () => import('@/features/account/pages/AccountSettingsPage.vue'),
        props: true,
        meta: { title: 'Account settings', icon: 'pi pi-user-edit' },
      },
      { path: ':path(.*)*', name: 'not-found', component: () => import('./pages/NotFoundPage.vue'), meta: { title: 'Not found', icon: 'pi pi-question-circle' } },
    ],
  },
]
