import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import SchedulingDiagnosticsModal from '../SchedulingDiagnosticsModal.vue'
import zh from '@/i18n/locales/zh/schedulingDiagnostics'
const { getSchedulingDiagnostics } = vi.hoisted(() => ({ getSchedulingDiagnostics: vi.fn() }))
vi.mock('@/api/admin/schedulingDiagnostics', () => ({ getSchedulingDiagnostics }))
const record = {
 id: 1, at: '2026-11-12T08:00:00Z', group_id: 6, model: 'test-model', layer: 'load_balance',
 selected_id: 2, top_k: 1, cost_enabled: true, sticky_weighted: true, truncated: false,
 accounts: [
 { id: 1, name: 'cheap', reason: 'model_not_supported', rate: null, score: null, ttft_ms: null, order: 0 },
 { id: 2, name: 'fast', rate: 0.03, rate_source: 'account', score: 8.5, parts: { cost: 2, latency: 1.5, sticky: 0.5 }, ttft_ms: 1200, error_rate: 0.1, latency_freshness: 0.5, order: 1 }
 ]
}
function create() {
 return mount(SchedulingDiagnosticsModal, {
 props: { show: true, group: '6', groups: [{ id: 6, name: 'test-group' }] },
 global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })],
 stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' } } }
 })
}
describe('SchedulingDiagnosticsModal', () => {
 beforeEach(() => { getSchedulingDiagnostics.mockReset(); getSchedulingDiagnostics.mockResolvedValue([record]) })
 it('shows real selections, score breakdown and translated exclusion reasons', async () => {
  const wrapper = create(); await flushPromises()
  expect(getSchedulingDiagnostics).toHaveBeenCalledWith(6)
  expect(wrapper.text()).toContain('账号不支持请求模型')
  expect(wrapper.text()).toContain('8.500')
  expect(wrapper.text()).toContain('1200 ms')
  expect(wrapper.text()).toContain('倍率 2.000')
  expect(wrapper.text()).toContain('账号保存倍率（手填/同步）')
  expect(wrapper.find('tbody tr.bg-emerald-50').text()).toContain('fast')
 })
 it('distinguishes empty records from a loading failure and allows retry', async () => {
  getSchedulingDiagnostics.mockRejectedValueOnce(new Error('failed'))
  const wrapper=create();await flushPromises()
  expect(wrapper.find('[role="alert"]').exists()).toBe(true)
  getSchedulingDiagnostics.mockResolvedValue([])
  await wrapper.find('button').trigger('click');await flushPromises()
  expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  expect(wrapper.text()).toContain('暂无调度记录')
 })
 it('does not present a direct sticky selection as a scored winner', async () => {
  getSchedulingDiagnostics.mockResolvedValue([{...record,layer:'previous_response_id',accounts:[],cost_enabled:false}])
  const wrapper=create();await flushPromises()
  expect(wrapper.text()).toContain('上次响应绑定')
  expect(wrapper.text()).toContain('本次未进入候选评分')
  expect(wrapper.text()).toContain('本次未启用倍率评分')
 })
 it('reloads when the group changes and ignores a response after close', async () => {
  const wrapper=create();await flushPromises()
  let resolve!: (data: typeof record[]) => void
  getSchedulingDiagnostics.mockReturnValueOnce(new Promise(r => { resolve=r }))
  await wrapper.find('select').setValue('')
  expect(getSchedulingDiagnostics).toHaveBeenLastCalledWith(undefined)
  await wrapper.setProps({show:false})
  resolve([]);await flushPromises()
  await wrapper.setProps({show:true});await flushPromises()
  expect(getSchedulingDiagnostics).toHaveBeenLastCalledWith(6)
 })
})
