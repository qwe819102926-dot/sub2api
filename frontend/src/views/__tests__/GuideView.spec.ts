import { describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

import GuideView from '../GuideView.vue'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

function mountGuide() {
  return mount(GuideView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
      },
    },
  })
}

describe('GuideView', () => {
  it('renders the local key智中转 usage guide', async () => {
    const wrapper = mountGuide()
    await flushPromises()
    const frame = wrapper.find('iframe.guide-frame')
    expect(frame.exists()).toBe(true)
    expect(frame.attributes('src')).toBe('/usage-guide/index.html')
    expect(frame.attributes('title')).toBe('key智中转使用说明')
  })

  it('sets the document title for the guide page', async () => {
    const wrapper = mountGuide()
    await flushPromises()
    expect(document.title).toBe('key智中转使用说明')
    wrapper.unmount()
  })
})
