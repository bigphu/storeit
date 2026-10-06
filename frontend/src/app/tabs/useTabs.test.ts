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
    ],
  })
  const app = createApp({})
  app.use(createPinia())
  app.use(router)
  const tabs = app.runWithContext(() => useTabs())
  router.afterEach((to, _from, failure) => {
    if (!failure) tabs.sync(to.fullPath, to.meta.title as string | undefined, undefined)
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
