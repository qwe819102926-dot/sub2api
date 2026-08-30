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
  it('renders the Codex tutorial markdown content', async () => {
    const wrapper = mountGuide()
    await flushPromises()
    expect(wrapper.text()).toContain('Codex 部署和使用教程')
    expect(wrapper.text()).toContain('教程目标')
    expect(wrapper.find('.markdown-page-content').exists()).toBe(true)
    expect(wrapper.find('.markdown-page-content').element.innerHTML).toContain('Codex 部署和使用教程')
  })

  it('builds a table of contents from the markdown headings', async () => {
    const wrapper = mountGuide()
    await flushPromises()
    const tocLinks = wrapper.findAll('.toc-item')
    expect(tocLinks.length).toBeGreaterThan(5)
  })
})
