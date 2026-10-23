import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, it, expect, vi } from 'vitest'
import RegistrationIPMonitor from '../RegistrationIPMonitor.vue'
const api = vi.hoisted(() => ({ getRegistrationIPReport: vi.fn(), saveRegistrationIPConfig: vi.fn(), unblockRegistrationIP: vi.fn() }))
vi.mock('@/api/admin/registrationIPMonitor', () => api)
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/utils/format', () => ({ formatDateTime: (value: string) => value }))
const fixture = () => ({ config: { enabled: true, window_hours: 24, max_accounts: 5, ban_hours: 24 }, rows: [{ ip: '192.0.2.1', total: 5, recent: 5, last_registered_at: '2026-10-23T12:00:00Z', blocked_at: '2026-10-23T12:05:00Z', blocked_until: '2026-10-24T12:05:00Z', active: true, ban_count: 1 }], accounts: [], total: 1, account_total: 0, page: 1 })
const button = (w: ReturnType<typeof mount>, key: string) => w.findAll('button').find(b => b.text() === key)!
describe('RegistrationIPMonitor', () => {
 beforeEach(() => { vi.clearAllMocks(); api.getRegistrationIPReport.mockResolvedValue(fixture()) })
 it('loads the rolling rule and opens accounts registered from a selected IP', async () => {
  const w = mount(RegistrationIPMonitor); await flushPromises()
  expect(w.get('input[type="number"]').element.value).toBe('24')
  expect(w.text()).toContain('registrationIPMonitor.blocked')
  api.getRegistrationIPReport.mockResolvedValue({ ...fixture(), account_total: 1, accounts: [{ user_id: 42, email: 'member@example.com', source: 'email', created_at: '2026-10-23T12:00:00Z', status: 'active', deleted: false }] })
  await button(w, 'registrationIPMonitor.accounts').trigger('click'); await flushPromises()
  expect(api.getRegistrationIPReport).toHaveBeenLastCalledWith({ search: '', ip: '192.0.2.1', page: 1 })
  expect(w.text()).toContain('member@example.com')
  await button(w, 'registrationIPMonitor.unblock').trigger('click'); await flushPromises()
  expect(api.unblockRegistrationIP).toHaveBeenCalledWith('192.0.2.1')
 })
 it('saves editable rule values through the dedicated endpoint', async () => {
  const w = mount(RegistrationIPMonitor); await flushPromises()
  await w.findAll('input[type="number"]')[1].setValue(3)
  await button(w, 'registrationIPMonitor.save').trigger('click'); await flushPromises()
  expect(api.saveRegistrationIPConfig).toHaveBeenCalledWith({ enabled: true, window_hours: 24, max_accounts: 3, ban_hours: 24 })
  expect(w.find('[role="status"]').text()).toBe('registrationIPMonitor.saved')
 })
 it('does not present failed monitoring requests as an empty registration history', async () => {
  api.getRegistrationIPReport.mockRejectedValueOnce(new Error('offline'))
  const w = mount(RegistrationIPMonitor); await flushPromises()
  expect(w.find('[role="alert"]').exists()).toBe(true)
  expect(w.text()).not.toContain('registrationIPMonitor.empty')
 })
})
