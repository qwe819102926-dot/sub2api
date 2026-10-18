import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, shallowMount } from '@vue/test-utils'
import { reactive, ref } from 'vue'
import ChannelStatusV2View from '../ChannelStatusV2View.vue'

const publicSettings = reactive({
  channel_monitor_hide_user_ranking: false,
  channel_monitor_hide_throughput: true,
})
const auth = reactive({ isAdmin: false })
const route = { query: { tab: 'users' } }
const api = vi.hoisted(() => ({
  getDimensions: vi.fn(), getSnapshot: vi.fn(), getMatrix: vi.fn(),
  getModels: vi.fn(), getErrors: vi.fn(), getUsers: vi.fn(),
}))
vi.mock('@/api/channelMonitorV2', () => api)
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: publicSettings, showError: vi.fn() }) }))
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => ({ replace: vi.fn() }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key, te: () => false, locale: ref('zh-CN') }),
}))
enableAutoUnmount(afterEach)
beforeEach(() => {
  vi.resetAllMocks()
  auth.isAdmin = false
  publicSettings.channel_monitor_hide_user_ranking = false
  route.query.tab = 'users'
  api.getDimensions.mockResolvedValue({ platforms: [], groups: [], models: [] })
  api.getSnapshot.mockResolvedValue(null)
  api.getMatrix.mockResolvedValue(null)
  api.getModels.mockResolvedValue({ items: [] })
  api.getErrors.mockResolvedValue({ items: [] })
  api.getUsers.mockResolvedValue({ items: [] })
})
function mountView() {
  return shallowMount(ChannelStatusV2View, {
    global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } },
  })
}
function rankingTab(wrapper: ReturnType<typeof mountView>) {
  return wrapper.findAll('[role="tab"]').find(tab => tab.text() === 'channelMonitorV2.tabs.users')
}
describe('channel monitor user ranking visibility', () => {
  it('removes the ranking tab and ignores a direct users-tab URL when hidden', async () => {
    publicSettings.channel_monitor_hide_user_ranking = true
    const wrapper = mountView()
    await flushPromises()
    expect(rankingTab(wrapper)).toBeUndefined()
    expect(api.getUsers).not.toHaveBeenCalled()
    expect(api.getModels).toHaveBeenCalled()
  })

  it('keeps ranking available to ordinary users by default', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(rankingTab(wrapper)?.attributes('aria-selected')).toBe('true')
    expect(api.getUsers).toHaveBeenCalledWith(expect.any(Object), false, expect.any(AbortSignal))
  })

  it('keeps administrators able to view ranking when it is hidden from users', async () => {
    publicSettings.channel_monitor_hide_user_ranking = true
    auth.isAdmin = true
    const wrapper = mountView()
    await flushPromises()
    expect(rankingTab(wrapper)?.attributes('aria-selected')).toBe('true')
    expect(api.getUsers).toHaveBeenCalledWith(expect.any(Object), true, expect.any(AbortSignal))
  })

  it('leaves an open ranking tab when settings change to hidden', async () => {
    const wrapper = mountView()
    await flushPromises()
    api.getUsers.mockClear()
    publicSettings.channel_monitor_hide_user_ranking = true
    await flushPromises()
    expect(rankingTab(wrapper)).toBeUndefined()
    expect(wrapper.get('[role="tab"]').attributes('aria-selected')).toBe('true')
    expect(api.getUsers).not.toHaveBeenCalled()
    expect(api.getModels).toHaveBeenCalled()
  })
})
