import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import InviteRewardsView from '../InviteRewardsView.vue'

const { getSignupPromotion, transferAffiliateQuota, refreshUser, showError, copyToClipboard } = vi.hoisted(() => ({
  getSignupPromotion: vi.fn(), transferAffiliateQuota: vi.fn(), refreshUser: vi.fn(), showError: vi.fn(), copyToClipboard: vi.fn(),
}))
vi.mock('@/api/user', () => ({ default: { getSignupPromotion, transferAffiliateQuota } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ refreshUser }) }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

const data = () => ({
  signup_bonus_enabled: true, signup_bonus_amount: 50, invite_bonus_enabled: true, invite_bonus_amount: 50,
  aff_code: 'FRIEND2026', invited_count: 21, total_bonus: 1050, page: 1, page_size: 20,
  records: [{ user_id: 2, email: 'f***@e***.com', created_at: '2026-10-21T01:00:00Z', amount: 50, status: 'credited' }],
  legacy_quota: 0, legacy_frozen_quota: 0,
})
const render = () => mount(InviteRewardsView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, RouterLink: true } } })
const button = (wrapper: ReturnType<typeof render>, label: string) => wrapper.findAll('button').find(b => b.text() === label)!

describe('InviteRewardsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getSignupPromotion.mockResolvedValue(data())
    refreshUser.mockResolvedValue(undefined)
    transferAffiliateQuota.mockResolvedValue({ transferred_quota: 8, balance: 8 })
  })

  it('shows invitation totals and copies a working referral link', async () => {
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[data-testid="invited-count"]').text()).toBe('21')
    expect(wrapper.get('[data-testid="total-bonus"]').text()).toContain('1,050')
    expect(wrapper.text()).toContain('f***@e***.com')
    expect(wrapper.get('[data-testid="total-bonus"]').text()).not.toContain('$')
    await button(wrapper, 'inviteRewards.copyLink').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith(`${window.location.origin}/register?aff=FRIEND2026`, 'inviteRewards.copied')
  })

  it('paginates records without calculating lifetime earnings from the visible page', async () => {
    const wrapper = render()
    await flushPromises()
    getSignupPromotion.mockResolvedValue({ ...data(), page: 2 })
    await button(wrapper, 'inviteRewards.next').trigger('click')
    await flushPromises()
    expect(getSignupPromotion).toHaveBeenLastCalledWith(2)
    expect(wrapper.get('[data-testid="total-bonus"]').text()).toContain('1,050')
  })

  it('keeps history and historical commission transfers available when rewards are paused', async () => {
    getSignupPromotion.mockResolvedValue({ ...data(), invite_bonus_enabled: false, legacy_quota: 8, legacy_frozen_quota: 5 })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).toContain('inviteRewards.paused')
    expect(button(wrapper, 'inviteRewards.copyLink').attributes('disabled')).toBeDefined()
    await button(wrapper, 'inviteRewards.legacyTransfer').trigger('click')
    await flushPromises()
    expect(transferAffiliateQuota).toHaveBeenCalledTimes(1)
    expect(refreshUser).toHaveBeenCalledTimes(1)
  })

  it('offers retry after a failed load', async () => {
    getSignupPromotion.mockRejectedValueOnce(new Error('offline'))
    const wrapper = render()
    await flushPromises()
    expect(showError).toHaveBeenCalled()
    await button(wrapper, 'inviteRewards.retry').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="total-bonus"]').exists()).toBe(true)
  })
})
