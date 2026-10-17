import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import OpsRequestDetailsModal from '../OpsRequestDetailsModal.vue'
import { resetAccountNameCacheForTests } from '../../utils/accountNameMap'

const { listRequestDetails, listAccounts, viewport } = vi.hoisted(() => ({
  listRequestDetails: vi.fn(),
  listAccounts: vi.fn(),
  viewport: { desktop: true },
}))

vi.mock('@vueuse/core', () => ({ useMediaQuery: () => ref(viewport.desktop) }))
vi.mock('@/api/admin/ops', () => ({ opsAPI: { listRequestDetails } }))
vi.mock('@/api/admin/accounts', () => ({ accountsAPI: { list: listAccounts } }))
vi.mock('@/api', () => ({ adminAPI: { groups: { getAll: vi.fn().mockResolvedValue([]) } } }))
vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: vi.fn() }),
  useAdminSettingsStore: () => ({ opsRealtimeMonitoringEnabled: false }),
}))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: vi.fn() }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

async function openDetails(sort: 'duration_desc' | 'ttft_desc') {
  const wrapper = mount(OpsRequestDetailsModal, {
    props: { modelValue: false, timeRange: '1h', preset: { title: 'Details', sort } },
    global: { stubs: { BaseDialog: { template: '<div><slot /></div>' }, Pagination: true } },
  })
  await wrapper.setProps({ modelValue: true })
  await flushPromises()
  return wrapper
}

describe('Ops request account names', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    resetAccountNameCacheForTests()
    viewport.desktop = true
    listRequestDetails.mockResolvedValue({
      items: [
        { kind: 'success', created_at: '2026-09-10T00:00:00Z', duration_ms: 12000, first_token_ms: 800, account_id: 101 },
        { kind: 'success', created_at: '2026-09-10T00:00:01Z', duration_ms: 9000, first_token_ms: 0, account_id: 102 },
        { kind: 'success', created_at: '2026-09-10T00:00:02Z', duration_ms: 5000, first_token_ms: null, account_id: 999 },
      ],
      total: 3,
    })
    listAccounts.mockResolvedValue({
      items: [
        { id: 101, name: 'acc-alpha' },
        { id: 102, name: 'acc-beta' },
      ],
      total: 2,
    })
  })

  it.each([true, false])('shows account names and missing-account fallback (desktop: %s)', async (desktop) => {
    viewport.desktop = desktop
    const wrapper = await openDetails('duration_desc')
    expect(wrapper.text()).toContain('acc-alpha')
    expect(wrapper.text()).toContain('acc-beta')
    expect(wrapper.text()).toContain('#999')
    wrapper.unmount()
  })

  it('shows account names with id fallback in the account column', async () => {
    const wrapper = await openDetails('duration_desc')
    expect(wrapper.text()).toContain('admin.ops.requestDetails.table.account')
    const accountCells = wrapper.findAll('tbody tr').map(tr => tr.findAll('td')[4].text())
    expect(accountCells).toEqual(['acc-alpha', 'acc-beta', '#999'])
    // 999 不在账号映射里，应触发一次强制刷新重建缓存
    expect(listAccounts).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('keeps total duration for duration details', async () => {
    const wrapper = await openDetails('duration_desc')
    expect(listRequestDetails).toHaveBeenCalledWith(expect.objectContaining({ sort: 'duration_desc' }))
    expect(wrapper.text()).toContain('admin.ops.requestDetails.table.duration')
    expect(wrapper.text()).toContain('12000 ms')
    expect(wrapper.text()).not.toContain('800 ms')
    wrapper.unmount()
  })
})
