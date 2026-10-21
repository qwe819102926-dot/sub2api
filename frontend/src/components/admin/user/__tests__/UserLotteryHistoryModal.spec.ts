import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import UserLotteryHistoryModal from '../UserLotteryHistoryModal.vue'
import type { AdminUser } from '@/types'

const { getLotteryHistory } = vi.hoisted(() => ({ getLotteryHistory: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { users: { getLotteryHistory } } }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string, values?: { id: number }) => values ? `${key}:${values.id}` : key })
}))

const user = { id: 7, email: 'user@example.test' } as AdminUser
const result = {
  items: [
    { id: 3, order_id: null, prize_amount: 7.5, is_winner: true, created_at: '2026-10-20T08:00:00Z' },
    { id: 2, order_id: 42, prize_amount: 0, is_winner: false, created_at: '2026-10-19T08:00:00Z' }
  ],
  total: 21, page: 1, page_size: 20, total_pages: 2, winning_draws: 5, total_prize_amount: 12.5
}
const mountModal = () => mount(UserLotteryHistoryModal, {
  props: { show: true, user },
  global: {
    stubs: {
      BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
      Pagination: { props: ['page'], template: '<button data-test="next-page" @click="$emit(\'update:page\', page + 1)">Next</button>' }
    }
  }
})

describe('UserLotteryHistoryModal', () => {
  beforeEach(() => {
    getLotteryHistory.mockReset()
    getLotteryHistory.mockResolvedValue(result)
  })

  it('shows lifetime bonus totals, individual wins, losses and chance sources', async () => {
    const wrapper = mountModal()
    await flushPromises()
    expect(getLotteryHistory).toHaveBeenCalledWith(7, 1, 20, { signal: expect.any(AbortSignal) })
    expect(wrapper.get('[data-test="lottery-total-prize"]').text()).toBe('12.50')
    const draws = wrapper.findAll('[data-test="lottery-draw"]')
    expect(draws).toHaveLength(2)
    expect(draws[0].text()).toContain('7.50')
    expect(draws[0].text()).toContain('admin.users.lotteryAdminSource')
    expect(draws[1].text()).toContain('admin.users.lotteryNotWon')
    expect(draws[1].text()).toContain('admin.users.lotteryOrderSource:42')
  })

  it('loads additional pages without changing lifetime totals', async () => {
    const wrapper = mountModal()
    await flushPromises()
    await wrapper.get('[data-test="next-page"]').trigger('click')
    await flushPromises()
    expect(getLotteryHistory).toHaveBeenLastCalledWith(7, 2, 20, { signal: expect.any(AbortSignal) })
    expect(wrapper.get('[data-test="lottery-total-prize"]').text()).toBe('12.50')
  })

  it('shows an explicit empty state for users without draws', async () => {
    getLotteryHistory.mockResolvedValue({ ...result, total: 0, items: [], winning_draws: 0, total_prize_amount: 0 })
    const wrapper = mountModal()
    await flushPromises()
    expect(wrapper.get('[data-test="lottery-empty"]').text()).toBe('admin.users.noLotteryHistory')
    expect(wrapper.get('[data-test="lottery-total-prize"]').text()).toBe('0.00')
  })

  it('shows an error instead of treating a failed request as zero winnings, and retries', async () => {
    getLotteryHistory.mockRejectedValueOnce(new Error('Network failure'))
    const wrapper = mountModal()
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('admin.users.failedToLoadLotteryHistory')
    expect(wrapper.get('[data-test="lottery-total-prize"]').text()).toBe('—')
    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-test="lottery-draw"]')).toHaveLength(2)
  })

  it('ignores a late response from the previously selected user', async () => {
    let resolveFirst!: (value: typeof result) => void
    getLotteryHistory.mockReturnValueOnce(new Promise(resolve => { resolveFirst = resolve }))
    const wrapper = mountModal()
    await wrapper.setProps({ user: { ...user, id: 8 } })
    await flushPromises()
    expect(wrapper.get('[data-test="lottery-total-prize"]').text()).toBe('12.50')
    resolveFirst({ ...result, total_prize_amount: 999 })
    await flushPromises()
    expect(wrapper.get('[data-test="lottery-total-prize"]').text()).toBe('12.50')
    expect(getLotteryHistory.mock.calls[0][3].signal.aborted).toBe(true)
  })
})
