import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { ref } from 'vue'
import DailyCheckinDialog from '../DailyCheckinDialog.vue'
import zh from '@/i18n/locales/zh/misc'
import type { DailyCheckinStatus } from '@/types/payment'

const mocks = vi.hoisted(() => ({ get: vi.fn(), claim: vi.fn(), refresh: vi.fn(), success: vi.fn(), error: vi.fn() }))
vi.mock('@/api/payment', () => ({ paymentAPI: { getDailyCheckin: mocks.get, claimDailyCheckin: mocks.claim } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 42 }, refreshUser: mocks.refresh }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: mocks.success, showError: mocks.error }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ locale: ref('zh-CN'), t: translate }) }))

function translate(key: string, params: Record<string, string | number> = {}) {
  let value: unknown = zh
  for (const part of key.split('.')) value = (value as Record<string, unknown>)?.[part]
  return Object.entries(params).reduce((text, [name, replacement]) => text.replaceAll(`{${name}}`, String(replacement)), typeof value === 'string' ? value : key)
}
function initialStatus(): DailyCheckinStatus {
  return {
    checked_in_today: false, has_recent_recharge: true, current_streak: 3, total_days: 3, total_reward: 40,
    cycle_day: 4, cycle_length: 30, today_reward: 0.2, reward_cycle: [],
    today: '2026-10-21', next_reset_at: '2026-10-22T00:00:00+08:00', month: '2026-10',
    ordinary_rewards: [0.2, 0.3], milestones: [{ day: 3, reward: 1 }, { day: 7, reward: 3 }, { day: 15, reward: 5 }, { day: 30, reward: 10 }],
    records: [{ date: '2026-10-20', streak_day: 3, reward: 20, created_at: '2026-10-20T01:00:00Z' }]
  }
}
let server: DailyCheckinStatus
let wrapper: VueWrapper | undefined
async function render() {
  wrapper = mount(DailyCheckinDialog, { attachTo: document.body })
  await flushPromises()
  return wrapper
}
async function open() { document.querySelector<HTMLButtonElement>('.ci-trigger')!.click(); await flushPromises() }
async function openDetails() { document.querySelector<HTMLButtonElement>('.ci-footer .ci-link')!.click(); await flushPromises() }
function claimedStatus(): DailyCheckinStatus {
  return { ...server, checked_in_today: true, current_streak: 4, total_days: 4, total_reward: 40.37, today_reward: 0.37,
    records: [...server.records, { date: '2026-10-21', streak_day: 4, reward: 0.37, created_at: '2026-10-21T04:00:00Z' }] }
}
beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-10-21T04:00:00Z'))
  vi.clearAllMocks()
  server = initialStatus()
  mocks.get.mockImplementation(async (month?: string) => ({ data: { ...server, month: month ?? server.month, records: month && month !== server.month ? [] : server.records } }))
  mocks.claim.mockImplementation(async () => { server = claimedStatus(); return { data: server } })
  mocks.refresh.mockResolvedValue(undefined)
})
afterEach(() => { wrapper?.unmount(); wrapper = undefined; document.body.innerHTML = ''; document.body.classList.remove('modal-open'); vi.useRealTimers() })

describe('daily check-in header and calendar', () => {
  it('uses the fixed milestone total while retaining historical credited amounts', async () => {
    server = { ...server, current_streak: 2, cycle_day: 3, today_reward: 1,
      records: [{ date: '2026-10-20', streak_day: 2, reward: 5, created_at: '2026-10-20T01:00:00Z' }] }
    await render(); await open()
    await openDetails()
    expect(document.querySelector('.ci-claim')?.textContent).toContain('+1赠金')
    expect(document.querySelector('.ci-milestone')?.textContent).toContain('+1当天到账')
    expect(document.querySelector('.ci-date[aria-label^="2026-10-21"]')?.textContent).toContain('+1')
    expect(document.querySelector('.ci-date[aria-label^="2026-10-20"]')?.textContent).toContain('+5')
    expect(document.querySelector('.ci-rules')?.textContent).toContain('普通日随机领取 0.20～0.30')
    expect(document.querySelector('.ci-rules')?.textContent).toContain('1、3、5、10')
    expect(document.querySelector('.ci-rules')?.textContent).toContain(zh.checkin.rechargeRequired)
  })
  it('disables claiming without a recent recharge and refreshes eligibility when reopened', async () => {
    server = { ...server, has_recent_recharge: false }
    await render(); await open()
    const button = document.querySelector<HTMLButtonElement>('.ci-claim .ci-action')!
    expect(button.disabled).toBe(true)
    expect(button.textContent?.trim()).toBe(zh.checkin.rechargeUnmet)
    expect(document.querySelector('.ci-claim')?.textContent).toContain('累计实付充值满 9.9 元')
    expect(document.querySelector('.ci-claim')?.textContent).toContain(zh.checkin.rechargeGiftExcluded)
    const qr = document.querySelector<HTMLImageElement>('.ci-trial-qr img')!
    expect(qr.alt).toBe(zh.checkin.trialQR)
    expect(qr.src).toContain('trial-credit-group-qq.png')
    expect(qr.closest('a')?.href).toBe(qr.src)
    button.click()
    expect(mocks.claim).not.toHaveBeenCalled()

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    server = { ...server, has_recent_recharge: true }
    await open()
    expect(document.querySelector<HTMLButtonElement>('.ci-claim .ci-action')?.disabled).toBe(false)
  })
  it('shows the recharge rule when eligibility expires after the dialog loads', async () => {
    await render(); await open()
    mocks.claim.mockRejectedValueOnce({ reason: 'CHECKIN_RECHARGE_REQUIRED' })
    server = { ...server, has_recent_recharge: false }
    document.querySelector<HTMLButtonElement>('.ci-claim .ci-action')!.click()
    await flushPromises()
    expect(mocks.error).toHaveBeenCalledWith(zh.checkin.rechargeRequired)
    expect(document.querySelector<HTMLButtonElement>('.ci-claim .ci-action')?.disabled).toBe(true)
  })
  it('shows the 30-day total and remaining recharge even after today’s check-in', async () => {
    server = { ...claimedStatus(), has_recent_recharge: false, recharge_progress: { total: 6.8, target: 9.9, remaining: 3.1 } }
    await render(); await open()
    expect(document.querySelector('.ci-callout h3')?.textContent).toBe(zh.checkin.checkedToday)
    const progress = document.querySelector<HTMLElement>('.ci-recharge')!
    expect(progress.textContent).toContain('近 30 天实付充值')
    expect(progress.querySelector('dd')?.textContent).toContain('6.8')
    expect(progress.querySelector('.ci-recharge-gap')?.textContent).toMatch(/还差.*3\.1/)
    expect(progress.querySelector('progress')?.value).toBe(6.8)
    expect(progress.querySelector('progress')?.max).toBe(9.9)
  })
  it('fills the progress bar and hides the gap once the target is reached', async () => {
    server = { ...server, recharge_progress: { total: 14.3, target: 9.9, remaining: 0 } }
    await render(); await open()
    const progress = document.querySelector<HTMLElement>('.ci-recharge')!
    expect(progress.querySelector('.ci-recharge-badge')?.textContent).toContain(zh.checkin.rechargeMet)
    expect(progress.querySelector('dd')?.textContent).toContain('14.3')
    expect(progress.querySelector('progress')?.value).toBe(9.9)
    expect(progress.querySelector('.ci-recharge-gap')).toBeNull()
  })
  it('opens a centered dialog and marks milestones by streak dates, including across months', async () => {
    await render()
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    await open()
    expect(document.querySelector('[role="dialog"]')?.getAttribute('aria-modal')).toBe('true')
    expect(document.querySelector('.ci-calendar')).toBeNull()
    await openDetails()
    expect(document.querySelector('.ci-weekday')?.textContent).toBe('一')
    expect(document.querySelector('.ci-date[aria-label^="2026-10-24"]')?.textContent).toContain('7 天')
    expect(document.querySelector('.ci-today-reward')).toBeNull()
    const node = document.querySelector<HTMLButtonElement>('.ci-milestone[aria-label*="2026-11-01"]')!
    node.click(); await flushPromises()
    expect(mocks.get).toHaveBeenLastCalledWith('2026-11')
    expect(document.querySelector('.ci-date[aria-label^="2026-11-01"]')?.textContent).toContain('+5')
    expect(document.querySelector('.ci-trigger')?.classList.contains('checked')).toBe(false)
  })
  it('credits once, shows the actual random reward, and retains the header badge after closing', async () => {
    await render(); await open()
    await openDetails()
    let resolveClaim!: (value: { data: DailyCheckinStatus }) => void
    mocks.claim.mockImplementation(() => new Promise(resolve => { resolveClaim = resolve }))
    const button = document.querySelector<HTMLButtonElement>('.ci-action')!
    button.click(); button.click()
    expect(mocks.claim).toHaveBeenCalledTimes(1)
    server = claimedStatus(); resolveClaim({ data: server }); await flushPromises()
    expect(document.querySelector('.ci-trigger')?.textContent).toContain('今日已签到')
    expect(document.querySelector('.ci-date[aria-label^="2026-10-21"]')?.textContent).toContain('+0.37')
    expect(document.querySelector('.ci-claim')?.textContent).toContain('+0.37')
    expect(document.querySelector('.ci-claim')?.textContent).not.toContain('～')
    expect(mocks.refresh).toHaveBeenCalledTimes(1)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })); await flushPromises()
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    expect(document.querySelector('.ci-trigger')?.textContent).toContain('今日已签到')
    expect(document.activeElement).toBe(document.querySelector('.ci-trigger'))
  })
  it('preserves the checked badge when browsing another month and reopens without another claim', async () => {
    server = claimedStatus()
    await render(); await open()
    await openDetails()
    document.querySelector<HTMLButtonElement>('.ci-milestone[aria-label*="2026-11-01"]')!.click(); await flushPromises()
    expect(document.querySelector('.ci-trigger')?.textContent).toContain('今日已签到')
    expect(document.querySelector<HTMLButtonElement>('.ci-action')?.disabled).toBe(true)
    expect(mocks.claim).not.toHaveBeenCalled()
  })
  it('resets the header state at the server-provided Shanghai midnight boundary', async () => {
    server = claimedStatus()
    await render()
    expect(document.querySelector('.ci-trigger')?.textContent).toContain('今日已签到')
    server = { ...server, checked_in_today: false, today: '2026-10-22', cycle_day: 5, next_reset_at: '2026-10-23T00:00:00+08:00' }
    await vi.advanceTimersByTimeAsync(12 * 3600000 + 1000)
    await flushPromises()
    expect(document.querySelector('.ci-trigger')?.classList.contains('checked')).toBe(false)
  })
  it('recovers a committed claim after a lost response instead of leaving the button claimable', async () => {
    await render(); await open()
    mocks.claim.mockImplementation(async () => { server = claimedStatus(); throw new Error('connection lost') })
    document.querySelector<HTMLButtonElement>('.ci-action')!.click(); await flushPromises()
    expect(document.querySelector('.ci-trigger')?.textContent).toContain('今日已签到')
    expect(document.querySelector<HTMLButtonElement>('.ci-action')?.disabled).toBe(true)
  })
})
