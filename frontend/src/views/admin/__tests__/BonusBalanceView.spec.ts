import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import BonusBalanceView from '../BonusBalanceView.vue'

const { getReport, getConversion, convertBalances } = vi.hoisted(() => ({ getReport: vi.fn(), getConversion: vi.fn(), convertBalances: vi.fn() }))
vi.mock('@/api/admin/bonusBalance', () => ({ getBonusBalanceReport: getReport, getBonusBalanceConversion: getConversion, convertBonusBalances: convertBalances }))
vi.mock('vue-i18n', async (original) => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key, locale: { value: 'en' } }) }))
const data = () => ({
  scope: 'current', started_at: '2026-10-23T00:00:00Z',
  summary: { grants: 60, users: 10, today_issued: 25, pending: 3, issued: 1000, consumed: 600, remaining: 300, adjustments: 50, historical_outflow: 50, deleted_user_remaining: 0, reconciliation_difference: 0 },
  sources: [{ source: 'signup', amount: 500, grants: 10, users: 10 }, { source: 'checkin', amount: 500, grants: 50, users: 10 }],
  records: [{ id: 1, user_id: 42, email: 'friend@example.com', source: 'checkin', amount: 10, balance_after: 30, historical: false, created_at: '2026-10-21T00:00:00Z', updated_at: '2026-10-21T00:00:00Z' }],
  total: 21, page: 1, page_size: 20,
})
const preview = () => ({ completed: false, users: 12, before_total: '300.00000000', after_total: '30.00000000', completed_at: null })
const render = () => mount(BonusBalanceView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' } } } })
const button = (w: ReturnType<typeof render>, label: string) => w.findAll('button').find(b => b.text() === label)!

describe('BonusBalanceView', () => {
  beforeEach(() => { vi.resetAllMocks(); getReport.mockImplementation(async (params) => ({ ...data(), scope: params.scope })); getConversion.mockResolvedValue(preview()) })
  it('requires explicit confirmation and runs only after submit, then permanently disables the button', async () => {
    convertBalances.mockResolvedValue({ ...preview(), completed: true, completed_at: '2026-10-22T00:00:00Z' })
    const w = render(); await flushPromises()
    await button(w, 'bonusBalanceAdmin.historyTab').trigger('click'); await flushPromises()
    await w.get('[data-testid="conversion-open"]').trigger('click'); await flushPromises()
    expect(convertBalances).not.toHaveBeenCalled()
    expect(w.get('[data-testid="conversion-submit"]').attributes('disabled')).toBeDefined()
    await w.get('#conversion-confirmation').setValue('DIVIDE_BY_10')
    await w.get('[data-testid="conversion-submit"]').trigger('click'); await flushPromises()
    expect(convertBalances).toHaveBeenCalledTimes(1)
    expect(convertBalances).toHaveBeenCalledWith('DIVIDE_BY_10')
    expect(w.get('[data-testid="conversion-open"]').attributes('disabled')).toBeDefined()
    expect(w.find('[data-testid="conversion-completed"]').exists()).toBe(true)
  })
  it('cancels without writing and reloads the preview when reopened', async () => {
    const w = render(); await flushPromises()
    await button(w, 'bonusBalanceAdmin.historyTab').trigger('click'); await flushPromises()
    await w.get('[data-testid="conversion-open"]').trigger('click'); await flushPromises()
    await button(w, 'common.cancel').trigger('click')
    expect(convertBalances).not.toHaveBeenCalled()
    await w.get('[data-testid="conversion-open"]').trigger('click'); await flushPromises()
    expect(getConversion).toHaveBeenCalledTimes(3)
  })
  it('reconciles a committed conversion after a network failure instead of allowing a second conversion', async () => {
    const w = render(); await flushPromises()
    await button(w, 'bonusBalanceAdmin.historyTab').trigger('click'); await flushPromises()
    await w.get('[data-testid="conversion-open"]').trigger('click'); await flushPromises()
    convertBalances.mockRejectedValueOnce(new Error('timeout'))
    getConversion.mockResolvedValue({ ...preview(), completed: true, completed_at: '2026-10-22T00:00:00Z' })
    await w.get('#conversion-confirmation').setValue('DIVIDE_BY_10')
    await w.get('[data-testid="conversion-submit"]').trigger('click'); await flushPromises()
    expect(w.get('[data-testid="conversion-open"]').attributes('disabled')).toBeDefined()
    expect(convertBalances).toHaveBeenCalledTimes(1)
  })
  it('does not open confirmation if the preview request fails', async () => {
    const w = render(); await flushPromises()
    await button(w, 'bonusBalanceAdmin.historyTab').trigger('click'); await flushPromises()
    getConversion.mockRejectedValueOnce(new Error('offline'))
    await w.get('[data-testid="conversion-open"]').trigger('click'); await flushPromises()
    expect(w.find('#conversion-confirmation').exists()).toBe(false)
    expect(convertBalances).not.toHaveBeenCalled()
    expect(w.find('[role="alert"]').exists()).toBe(true)
  })
  it('shows global issuance, measured consumption and unconsumed stock', async () => {
    const w = render(); await flushPromises()
    await button(w, 'bonusBalanceAdmin.historyTab').trigger('click'); await flushPromises()
    expect(w.get('[data-testid="issued"]').text()).toContain('1,000')
    expect(w.get('[data-testid="consumed"]').text()).toContain('600')
    expect(w.get('[data-testid="remaining"]').text()).toContain('300')
    expect(w.text()).toContain('bonusBalanceAdmin.sources.signup')
    expect(w.text()).toContain('bonusBalanceAdmin.sources.checkin')
    expect(w.text()).toContain('friend@example.com')
  })
  it('passes source filters to the server for consistent totals and records', async () => {
    const w = render(); await flushPromises()
    await button(w, 'bonusBalanceAdmin.sources.signup').trigger('click'); await flushPromises()
    expect(getReport).toHaveBeenLastCalledWith(expect.objectContaining({ source: 'signup', kind: 'grant', page: 1 }))
    expect(w.get('[data-testid="currentIssued"]').text()).toContain('1,000')
  })
  it('defaults to the new reporting period and keeps unknown credits separate', async () => {
    const w = render(); await flushPromises()
    expect(getReport).toHaveBeenCalledWith(expect.objectContaining({ scope: 'current' }))
    expect(w.get('[data-testid="currentIssued"]').text()).toContain('1,000')
    expect(w.get('[data-testid="recipientCount"]').text()).toBe('10')
    expect(w.get('[data-testid="grantCount"]').text()).toBe('60')
    expect(w.get('[data-testid="pending"]').text()).toContain('3')
    expect(w.find('[data-testid="remaining"]').exists()).toBe(false)
    expect(w.find('[data-testid="conversion-open"]').exists()).toBe(false)
    expect(w.find('[data-testid="epoch-hint"]').exists()).toBe(true)
    await w.get('input[type="date"]').setValue('2026-10-23')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(getReport).toHaveBeenLastCalledWith(expect.objectContaining({ start_date: '2026-10-23' }))
    await button(w, 'bonusBalanceAdmin.sources.checkin').trigger('click'); await flushPromises()
    await button(w, 'bonusBalanceAdmin.historyTab').trigger('click'); await flushPromises()
    expect(getReport).toHaveBeenLastCalledWith(expect.objectContaining({ scope: 'history', source: '', start_date: '', page: 1 }))
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
    expect(w.find('[data-testid="currentIssued"]').exists()).toBe(false)
    await button(w, 'bonusBalanceAdmin.retry').trigger('click'); await flushPromises()
    expect(w.find('[data-testid="currentIssued"]').exists()).toBe(true)
  })
})
