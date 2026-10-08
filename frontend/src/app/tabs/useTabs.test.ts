import { createPinia } from 'pinia'
import { describe, expect, it, vi } from 'vitest'
import { createApp } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { useTabs } from './useTabs'

// usePreferences đặt class theme lên <html>; môi trường test là node, không có DOM
vi.stubGlobal('document', { documentElement: { classList: { toggle: () => {} } } })

// Store tab với router thật (bộ nhớ). afterEach thay cho watch(route.fullPath) của
// AppLayout; điều hướng tới đúng URL đang mở vẫn gọi afterEach (kèm lỗi "duplicated")
// nhưng URL không đổi nên watch không chạy, nên ở đây bỏ qua các lần điều hướng hỏng.
async function setup() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/assets', component: {}, meta: { title: 'Assets' } },
      { path: '/statuses', component: {}, meta: { title: 'Statuses' } },
      { path: '/accounts', component: {}, meta: { title: 'Accounts' } },
      { path: '/accounts/:id', component: {}, meta: { title: 'Account' } },
      { path: '/empty', component: {}, meta: { tab: false } },
    ],
  })
  const app = createApp({})
  app.use(createPinia())
  app.use(router)
  const tabs = app.runWithContext(() => useTabs())
  router.afterEach((to, _from, failure) => {
    if (!failure) tabs.sync(to.fullPath, to.meta.title as string | undefined, undefined, to.meta.tab !== false)
  })
  await router.push('/assets')
  return { tabs, router }
}

describe('useTabs', () => {
  it('gives the page to the tab you switch to, even when both tabs show the same URL', async () => {
    const { tabs } = await setup()
    const first = tabs.tabs[0].id
    await tabs.duplicate(first)
    const copy = tabs.activeId!
    expect(copy).not.toBe(first)
    // trang (KeepAlive, tiêu đề, lịch sử) thuộc tab đang chọn
    expect(tabs.routeTabId).toBe(copy)

    await tabs.activate(first)
    expect(tabs.activeId).toBe(first)
    expect(tabs.routeTabId).toBe(first)
  })

  it('titles the tab after the section it moves to', async () => {
    const { tabs, router } = await setup()
    const id = tabs.activeId!
    await router.push('/statuses')
    expect(tabs.byId(id)?.title).toBe('Statuses')
    expect(tabs.routeTabId).toBe(id)
  })
})

describe('useTabs back and forth (probe)', () => {
  it('list tab and account tab: switching keeps highlight and page together', async () => {
    const { tabs, router } = await setup()
    await router.push('/accounts')
    const list = tabs.activeId!
    await tabs.open('/accounts/42', { background: false })
    const acct = tabs.activeId!
    for (let i = 0; i < 3; i++) {
      await tabs.activate(list)
      expect([tabs.activeId, tabs.routeTabId, router.currentRoute.value.fullPath]).toEqual([list, list, '/accounts'])
      await tabs.activate(acct)
      expect([tabs.activeId, tabs.routeTabId, router.currentRoute.value.fullPath]).toEqual([acct, acct, '/accounts/42'])
    }
  })

  it('one tab, browser back and forward between list and account', async () => {
    const { tabs, router } = await setup()
    await router.push('/accounts')
    const id = tabs.activeId!
    await router.push('/accounts/42')
    for (let i = 0; i < 2; i++) {
      router.back()
      await new Promise((r) => setTimeout(r, 10))
      expect([tabs.activeId, tabs.byId(id)?.path, tabs.byId(id)?.title]).toEqual([id, '/accounts', 'Accounts'])
      router.forward()
      await new Promise((r) => setTimeout(r, 10))
      expect([tabs.activeId, tabs.byId(id)?.path, tabs.byId(id)?.title]).toEqual([id, '/accounts/42', 'Account'])
    }
  })

  it('one tab, app back (goBack) then open the account again', async () => {
    const { tabs, router } = await setup()
    await router.push('/accounts')
    const id = tabs.activeId!
    await router.push('/accounts/42')
    await tabs.goBack()
    expect([tabs.byId(id)?.path, tabs.byId(id)?.title]).toEqual(['/accounts', 'Accounts'])
    await router.push('/accounts/42')
    expect([tabs.byId(id)?.path, tabs.byId(id)?.title]).toEqual(['/accounts/42', 'Account'])
  })

  it('account tab already open, opening it again from the list tab', async () => {
    const { tabs, router } = await setup()
    await router.push('/accounts')
    const list = tabs.activeId!
    await tabs.open('/accounts/42', { background: false })
    const acct = tabs.activeId!
    await tabs.activate(list)
    // bấm dòng trong danh sách: đi trong tab danh sách tới đúng URL của tab kia
    await router.push('/accounts/42')
    expect([tabs.activeId, tabs.routeTabId]).toEqual([list, list])
    await tabs.activate(acct)
    expect([tabs.activeId, tabs.routeTabId]).toEqual([acct, acct])
    await tabs.activate(list)
    expect([tabs.activeId, tabs.routeTabId]).toEqual([list, list])
  })
})

describe('closing every tab', () => {
  it('leaves no tabs and shows /empty; the next page opens a tab; Undo brings the tab back', async () => {
    const { tabs, router } = await setup()
    const only = tabs.tabs[0]
    await tabs.close(only.id)
    expect(tabs.tabs).toHaveLength(0)
    expect(router.currentRoute.value.path).toBe('/empty')
    expect(tabs.activeId).toBeNull()

    await router.push('/statuses')
    expect(tabs.tabs.map((t) => t.path)).toEqual(['/statuses'])
    expect(tabs.activeId).toBe(tabs.tabs[0].id)

    await tabs.close(tabs.tabs[0].id)
    await tabs.reopen(only, 0, true)
    expect(tabs.tabs.map((t) => t.path)).toEqual(['/assets'])
    expect(router.currentRoute.value.path).toBe('/assets')
  })

  it('never turns /empty into a tab', async () => {
    const { tabs, router } = await setup()
    await router.push('/empty')
    expect(tabs.tabs.map((t) => t.path)).toEqual(['/assets'])
  })
})
