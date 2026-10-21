import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UserBonusBalanceModal from '../UserBonusBalanceModal.vue'

const { updateBonusBalance, showSuccess, showError } = vi.hoisted(() => ({
  updateBonusBalance: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: { updateBonusBalance }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError })
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({
    t: (key: string) => key
  })
}))

const mountModal = () => mount(UserBonusBalanceModal, {
  props: {
    show: true,
    user: {
      id: 7,
      email: 'user@example.test',
      bonus_balance: 12.5
    } as never
  },
  global: {
    stubs: {
      BaseDialog: {
        props: ['show', 'title'],
        template: '<div v-if="show"><slot /><slot name="footer" /></div>'
      }
    }
  }
})

describe('UserBonusBalanceModal', () => {
  beforeEach(() => {
    updateBonusBalance.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    updateBonusBalance.mockResolvedValue({})
  })

  it('previews the bonus balance after an increase', async () => {
    const wrapper = mountModal()

    await wrapper.get('[data-test="bonus-amount-input"]').setValue('2.5')
    await flushPromises()

    expect(wrapper.get('[data-test="bonus-preview"]').text()).toContain('15.00')
    expect(wrapper.get('[data-test="bonus-submit"]').text()).toBe('admin.users.adjustBonusBalanceAction')
  })

  it('blocks a subtract that would go negative', async () => {
    const wrapper = mountModal()

    await wrapper.get('[data-test="bonus-operation-subtract"]').trigger('click')
    await wrapper.get('[data-test="bonus-amount-input"]').setValue('20')
    await wrapper.get('#bonus-balance-form').trigger('submit')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('admin.users.insufficientBonusBalance')
    expect(updateBonusBalance).not.toHaveBeenCalled()
  })

  it('submits an add adjustment', async () => {
    const wrapper = mountModal()

    await wrapper.get('[data-test="bonus-amount-input"]').setValue('3')
    await wrapper.get('#bonus-balance-form').trigger('submit')
    await flushPromises()

    expect(updateBonusBalance).toHaveBeenCalledWith(7, 3, 'add', '')
    expect(showSuccess).toHaveBeenCalledWith('admin.users.adjustBonusBalanceSuccess')
    expect(wrapper.emitted('success')).toBeTruthy()
  })

  it('does not treat a missing bonus_balance as zero', async () => {
    const wrapper = mount(UserBonusBalanceModal, {
      props: {
        show: true,
        user: {
          id: 7,
          email: 'user@example.test'
        } as never
      },
      global: {
        stubs: {
          BaseDialog: {
            props: ['show', 'title'],
            template: '<div v-if="show"><slot /><slot name="footer" /></div>'
          }
        }
      }
    })

    expect(wrapper.get('[data-test="current-bonus-balance"]').text()).toBe('—')
    expect(wrapper.get('[data-test="bonus-operation-subtract"]').attributes('disabled')).toBeDefined()

    await wrapper.get('[data-test="bonus-amount-input"]').setValue('3')
    await flushPromises()
    expect(wrapper.find('[data-test="bonus-preview"]').exists()).toBe(false)

    await wrapper.get('#bonus-balance-form').trigger('submit')
    await flushPromises()
    expect(updateBonusBalance).toHaveBeenCalledWith(7, 3, 'add', '')
  })
})
