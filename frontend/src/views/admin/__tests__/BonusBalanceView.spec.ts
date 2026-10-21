import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import BonusBalanceView from '../BonusBalanceView.vue'

const { getReport } = vi.hoisted(() => ({ getReport: vi.fn() }))
vi.mock('@/api/admin/bonusBalance', () => ({ getBonusBalanceReport: getReport }))
vi.mock('vue-i18n', async (original) => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
const data = () => ({
  summary: { issued: 1000, consumed: 600, remaining: 300, adjustments: 50, historical_outflow: 50, deleted_user_remaining: 0, reconciliation_difference: 0 },
  sources: [{ source: 'signup', amount: 500, grants: 10, users: 10 }, { source: 'checkin', amount: 500, grants: 50, users: 10 }],
  records: [{ id: 1, user_id: 42, email: 'friend@example.com', source: 'checkin', amount: 10, balance_after: 30, historical: false, created_at: '2026-10-21T00:00:00Z', updated_at: '2026-10-21T00:00:00Z' }],
  total: 21, page: 1, page_size: 20,
})
const render = () => mount(BonusBalanceView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' } } } })
const button = (w: ReturnType<typeof render>, label: string) => w.findAll('button').find(b => b.text() === label)!

describe('BonusBalanceView', () => {
  beforeEach(() => { vi.clearAllMocks(); getReport.mockResolvedValue(data()) })
  it('shows global issuance, measured consumption and unconsumed stock', async () => {
    const w = render(); await flushPromises()
    expect(w.get('[data-testid="issued"]').text()).toContain('1,000')
    expect(w.get('[data-testid="consumed"]').text()).toContain('600')
    expect(w.get('[data-testid="remaining"]').text()).toContain('300')
    expect(w.text()).not.toContain('$')
    expect(w.text()).toContain('bonusBalanceAdmin.sources.signup')
    expect(w.text()).toContain('bonusBalanceAdmin.sources.checkin')
    expect(w.text()).toContain('friend@example.com')
  })
  it('filters to a campaign without recalculating global totals from a page of records', async () => {
    const w = render(); await flushPromises()
    await button(w, 'bonusBalanceAdmin.sources.signup').trigger('click'); await flushPromises()
    expect(getReport).toHaveBeenLastCalledWith(expect.objectContaining({ source: 'signup', kind: 'grant', page: 1 }))
    expect(w.get('[data-testid="issued"]').text()).toContain('1,000')
  })
  it('passes user searches and paginates', async () => {
    const w = render(); await flushPromises()
    await w.get('input').setValue('42'); await w.get('form').trigger('submit'); await flushPromises()
    expect(getReport).toHaveBeenLastCalledWith(expect.objectContaining({ search: '42', page: 1 }))
    getReport.mockResolvedValue({ ...data(), page: 2 })
    await button(w, 'pagination.next').trigger('click'); await flushPromises()
    expect(getReport).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2 }))
  })
  it('shows an explicit error and retries rather than presenting a failed load as zero', async () => {
    getReport.mockRejectedValueOnce(new Error('offline'))
    const w = render(); await flushPromises()
    expect(w.get('[role="alert"]').exists()).toBe(true)
    expect(w.find('[data-testid="issued"]').exists()).toBe(false)
    await button(w, 'bonusBalanceAdmin.retry').trigger('click'); await flushPromises()
    expect(w.find('[data-testid="issued"]').exists()).toBe(true)
  })
})
