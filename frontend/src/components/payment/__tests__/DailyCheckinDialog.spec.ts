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
vi.mock('vue-i18n', async (original) => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ locale: ref('zh-CN'), t: translate }) }))

function translate(key: string, params: Record<string, string | number> = {}) {
  let value: unknown = zh
  for (const part of key.split('.')) value = (value as Record<string, unknown>)?.[part]
  return Object.entries(params).reduce((text, [name, replacement]) => text.replaceAll(`{${name}}`, String(replacement)), typeof value === 'string' ? value : key)
}
function initialStatus(): DailyCheckinStatus {
  return {
    checked_in_today: false, current_streak: 3, total_days: 3, total_reward: 40,
    cycle_day: 4, cycle_length: 30, today_reward: 10, reward_cycle: [],
    today: '2026-10-21', next_reset_at: '2026-10-22T00:00:00+08:00', month: '2026-10',
    ordinary_rewards: [10], milestones: [{ day: 3, reward: 20 }, { day: 7, reward: 30 }, { day: 15, reward: 50 }, { day: 30, reward: 100 }],
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
function claimedStatus(): DailyCheckinStatus {
  return { ...server, checked_in_today: true, current_streak: 4, total_days: 4, total_reward: 50, today_reward: 10,
    records: [...server.records, { date: '2026-10-21', streak_day: 4, reward: 10, created_at: '2026-10-21T04:00:00Z' }] }
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
    server = { ...server, current_streak: 2, cycle_day: 3, today_reward: 20,
      records: [{ date: '2026-10-20', streak_day: 2, reward: 0.5, created_at: '2026-10-20T01:00:00Z' }] }
    await render(); await open()
    expect(document.querySelector('.ci-claim')?.textContent).toContain('+20')
    expect(document.querySelector('.ci-milestone')?.textContent).toContain('+20当天到账')
    expect(document.querySelector('.ci-date[aria-label^="2026-10-21"]')?.textContent).toContain('+20')
    expect(document.querySelector('.ci-date[aria-label^="2026-10-20"]')?.textContent).toContain('+0.50')
  })
  it('opens a centered dialog and marks milestones by streak dates, including across months', async () => {
    await render()
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    await open()
    expect(document.querySelector('[role="dialog"]')?.getAttribute('aria-modal')).toBe('true')
    expect(document.querySelector('.ci-weekday')?.textContent).toBe('一')
    expect(document.querySelector('.ci-date[aria-label^="2026-10-24"]')?.textContent).toContain('7 天')
    expect(document.querySelector('.ci-claim')?.textContent).toContain('+10')
    const node = document.querySelector<HTMLButtonElement>('.ci-milestone[aria-label*="2026-11-01"]')!
    node.click(); await flushPromises()
    expect(mocks.get).toHaveBeenLastCalledWith('2026-11')
    expect(document.querySelector('.ci-date[aria-label^="2026-11-01"]')?.textContent).toContain('+50')
    expect(document.querySelector('.ci-trigger')?.classList.contains('checked')).toBe(false)
  })
  it('credits once, shows the fixed daily reward, and retains the header badge after closing', async () => {
    await render(); await open()
    let resolveClaim!: (value: { data: DailyCheckinStatus }) => void
    mocks.claim.mockImplementation(() => new Promise(resolve => { resolveClaim = resolve }))
    const button = document.querySelector<HTMLButtonElement>('.ci-action')!
    button.click(); button.click()
    expect(mocks.claim).toHaveBeenCalledTimes(1)
    server = claimedStatus(); resolveClaim({ data: server }); await flushPromises()
    expect(document.querySelector('.ci-trigger')?.textContent).toContain('今日已签到')
    expect(document.querySelector('.ci-date[aria-label^="2026-10-21"]')?.textContent).toContain('+10')
    expect(mocks.refresh).toHaveBeenCalledTimes(1)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })); await flushPromises()
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    expect(document.querySelector('.ci-trigger')?.textContent).toContain('今日已签到')
    expect(document.activeElement).toBe(document.querySelector('.ci-trigger'))
  })
  it('preserves the checked badge when browsing another month and reopens without another claim', async () => {
    server = claimedStatus()
    await render(); await open()
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
