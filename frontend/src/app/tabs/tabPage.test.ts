import { createPinia } from 'pinia'
import { describe, expect, it, vi } from 'vitest'
import { createApp, effectScope, nextTick, ref } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { useTabTitle } from './tabPage'
import { useTabs } from './useTabs'

// usePreferences đặt class theme lên <html>; môi trường test là node, không có DOM
vi.stubGlobal('document', { documentElement: { classList: { toggle: () => {} } } })
// onActivated/onDeactivated gọi ngoài component chỉ cảnh báo; trang coi như đang hiện
vi.spyOn(console, 'warn').mockImplementation(() => {})

describe('useTabTitle', () => {
  // Rời trang trong cùng tab (Back): sync đặt tiêu đề theo route mới trước khi trang cũ
  // bị KeepAlive cất đi. Trang cũ không được ghi lại tiêu đề của nó lên tab.
  it('does not put the page title back when the tab moves to another section', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/accounts', component: {}, meta: { title: 'Accounts' } },
        { path: '/accounts/:id', component: {}, meta: { title: 'Account' } },
      ],
    })
    const app = createApp({})
    app.use(createPinia())
    app.use(router)
    const tabs = app.runWithContext(() => useTabs())
    router.afterEach((to, _from, failure) => {
      if (!failure) tabs.sync(to.fullPath, to.meta.title as string | undefined, undefined)
    })
    await router.push('/accounts/42')
    const id = tabs.activeId!

    // trang tài khoản đặt tiêu đề theo tên
    const name = ref('Lê Văn Hùng')
    const scope = effectScope()
    app.runWithContext(() => scope.run(() => useTabTitle(() => name.value)))
    await nextTick()
    expect(tabs.byId(id)?.title).toBe('Lê Văn Hùng')

    // Back về danh sách: tiêu đề theo route mới, trang tài khoản không được ghi đè
    await router.push('/accounts')
    await nextTick()
    expect(tabs.byId(id)?.title).toBe('Accounts')

    // tên đổi khi trang vẫn hiện (dữ liệu tải xong) thì tiêu đề vẫn theo trang
    scope.stop()
  })

  it('follows the page title while the page is shown', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/accounts/:id', component: {}, meta: { title: 'Account' } }] })
    const app = createApp({})
    app.use(createPinia())
    app.use(router)
    const tabs = app.runWithContext(() => useTabs())
    router.afterEach((to) => tabs.sync(to.fullPath, to.meta.title as string | undefined, undefined))
    await router.push('/accounts/42')
    const id = tabs.activeId!
    const name = ref<string | undefined>(undefined)
    const scope = effectScope()
    app.runWithContext(() => scope.run(() => useTabTitle(() => name.value)))
    name.value = 'Lê Văn Hùng'
    await nextTick()
    expect(tabs.byId(id)?.title).toBe('Lê Văn Hùng')
    name.value = 'Lê Văn Hùng (Kho)'
    await nextTick()
    expect(tabs.byId(id)?.title).toBe('Lê Văn Hùng (Kho)')
    scope.stop()
  })
})
