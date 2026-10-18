import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import RedeemView from '../RedeemView.vue'

const { redeem, getHistory, refreshUser, fetchActiveSubscriptions, showError, showWarning, showSuccess } = vi.hoisted(() => ({
  redeem: vi.fn(),
  getHistory: vi.fn(),
  refreshUser: vi.fn(),
  fetchActiveSubscriptions: vi.fn(),
  showError: vi.fn(),
  showWarning: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api', () => ({
  redeemAPI: { redeem, getHistory },
  authAPI: { getPublicSettings: vi.fn().mockResolvedValue({}) },
}))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: { balance: 10, concurrency: 2 }, refreshUser }),
}))
vi.mock('@/stores/subscriptions', () => ({
  useSubscriptionStore: () => ({ fetchActiveSubscriptions }),
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showWarning, showSuccess }),
}))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

async function submitCode() {
  const wrapper = mount(RedeemView, {
    global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true } },
  })
  await flushPromises()
  await wrapper.get('input#code').setValue(' REDEEM-CODE ')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  return wrapper
}

describe('RedeemView paginated history', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    redeem.mockResolvedValue({ type: 'balance', value: 20, message: 'Code applied' })
    getHistory.mockResolvedValue({ items: [], total: 0 })
    refreshUser.mockResolvedValue({ balance: 30, concurrency: 2 })
    fetchActiveSubscriptions.mockResolvedValue([])
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('pages on the server, changes size, and resets page and total after redeeming', async () => {
    getHistory.mockResolvedValue({ items: [], total: 101 })
    const wrapper = mount(RedeemView, {
      global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true } },
    })
    await flushPromises()
    const button = (text: string) => wrapper.findAll('button').find(b => b.text() === text)!
    expect(getHistory).toHaveBeenLastCalledWith(1, 20)
    expect(button('pagination.previous').attributes('disabled')).toBeDefined()
    await button('pagination.next').trigger('click')
    await flushPromises()
    expect(getHistory).toHaveBeenLastCalledWith(2, 20)
    await button('pagination.previous').trigger('click')
    await flushPromises()
    expect(getHistory).toHaveBeenLastCalledWith(1, 20)
    expect(wrapper.findAll('select option').map(o => o.text())).toEqual(['20', '50', '100'])
    await wrapper.get('select').setValue('50')
    await flushPromises()
    expect(getHistory).toHaveBeenLastCalledWith(1, 50)
    await wrapper.get('select').setValue('100')
    await flushPromises()
    await button('pagination.next').trigger('click')
    await flushPromises()
    expect(getHistory).toHaveBeenLastCalledWith(2, 100)
    expect(button('pagination.next').attributes('disabled')).toBeDefined()
    getHistory.mockResolvedValue({ items: [], total: 102 })
    await wrapper.get('input#code').setValue('NEW-CODE')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(getHistory).toHaveBeenLastCalledWith(1, 100)
    expect(wrapper.text()).toContain('102')
    expect(button('pagination.previous').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('restores the loaded size and keeps rows and navigation usable after a size request fails', async () => {
    const item = { id: 1, code: 'OLD-ROWS', type: 'balance', value: 20, used_at: '2026-03-08T00:00:00Z' }
    getHistory.mockResolvedValue({ items: [item], total: 61 })
    const wrapper = mount(RedeemView, {
      global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true } },
    })
    await flushPromises()
    const button = (text: string) => wrapper.findAll('button').find(b => b.text() === text)!
    await button('pagination.next').trigger('click')
    await flushPromises()
    let rejectRequest!: (error: Error) => void
    getHistory.mockImplementationOnce(() => new Promise((_resolve, reject) => { rejectRequest = reject }))
    await wrapper.get('select').setValue('50')
    expect(getHistory).toHaveBeenLastCalledWith(1, 50)
    expect(button('pagination.next').attributes('disabled')).toBeDefined()
    rejectRequest(new Error('Network error'))
    await flushPromises()
    expect(wrapper.get('select').element.value).toBe('20')
    expect(wrapper.text()).toContain('OLD-ROWS')
    expect(wrapper.text()).toContain('61')
    expect(button('pagination.previous').attributes('disabled')).toBeUndefined()
    expect(button('pagination.next').attributes('disabled')).toBeUndefined()
    expect(showError).toHaveBeenCalledWith('redeem.historyLoadFailed')
    await button('pagination.next').trigger('click')
    await flushPromises()
    expect(getHistory).toHaveBeenLastCalledWith(3, 20)
    await wrapper.get('select').setValue('50')
    await flushPromises()
    expect(getHistory).toHaveBeenLastCalledWith(1, 50)
    expect(wrapper.get('select').element.value).toBe('50')
    expect(button('pagination.previous').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it.each(['success', 'failure'])('ignores a stale history %s after a newer size request succeeds', async (outcome) => {
    let resolveOld!: (value: unknown) => void
    let rejectOld!: (error: Error) => void
    getHistory.mockImplementationOnce(() => new Promise((resolve, reject) => {
      resolveOld = resolve
      rejectOld = reject
    }))
    const wrapper = mount(RedeemView, {
      global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true } },
    })
    getHistory.mockResolvedValue({ items: [{
      id: 2, code: 'NEW-ROWS', type: 'balance', value: 30, used_at: '2026-03-08T00:00:00Z',
    }], total: 61 })
    // Force overlapping requests to exercise responses arriving out of order.
    await wrapper.get('select').setValue('50')
    await flushPromises()
    const button = (text: string) => wrapper.findAll('button').find(b => b.text() === text)!
    await button('pagination.next').trigger('click')
    await flushPromises()
    expect(getHistory).toHaveBeenLastCalledWith(2, 50)
    if (outcome === 'success') {
      resolveOld({ items: [], total: 0 })
    } else {
      rejectOld(new Error('Stale network error'))
    }
    await flushPromises()
    expect(wrapper.get('select').element.value).toBe('50')
    expect(wrapper.text()).toContain('NEW-ROWS')
    expect(wrapper.text()).toContain('61')
    expect(button('pagination.previous').attributes('disabled')).toBeUndefined()
    expect(button('pagination.next').attributes('disabled')).toBeDefined()
    expect(wrapper.get('select').attributes('disabled')).toBeUndefined()
    expect(showError).not.toHaveBeenCalled()
    await button('pagination.previous').trigger('click')
    await flushPromises()
    expect(getHistory).toHaveBeenLastCalledWith(1, 50)
    wrapper.unmount()
  })

  it('disables both paging buttons for empty history', async () => {
    const wrapper = await submitCode()
    for (const button of wrapper.findAll('button').filter(b => b.text().startsWith('pagination.'))) {
      expect(button.attributes('disabled')).toBeDefined()
    }
    wrapper.unmount()
  })

  it('keeps administrator bonus adjustments visible in paginated history', async () => {
    getHistory.mockResolvedValue({ items: [{
      id: 1, code: 'BONUS-ENTRY', type: 'admin_bonus', value: 10,
      notes: 'Bonus adjustment', used_at: '2026-03-08T00:00:00Z',
    }], total: 1 })
    const wrapper = mount(RedeemView, {
      global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true } },
    })
    await flushPromises()
    expect(wrapper.text()).toContain('redeem.bonusBalanceAddedAdmin')
    expect(wrapper.text()).toContain('Bonus adjustment')
    expect(wrapper.text()).toContain('1')
    wrapper.unmount()
  })

})
