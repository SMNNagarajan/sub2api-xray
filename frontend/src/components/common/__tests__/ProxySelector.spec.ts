import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import ProxySelector from '../ProxySelector.vue'
import type { Proxy } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key === 'myResources.states.publicDetailsHidden'
        ? 'Public proxy hidden'
        : key
    })
  }
})

const hiddenPublicProxy = {
  id: 2,
  owner_user_id: null,
  is_public: true,
  is_owned: false,
  details_hidden: true,
  name: 'Shared proxy',
  kind: 'standard',
  protocol: 'http',
  host: '',
  port: 0,
  username: null,
  status: 'active',
  expires_at: null,
  fallback_mode: 'none',
  expiry_warn_days: 7,
  created_at: '2026-07-20T00:00:00Z',
  updated_at: '2026-07-20T00:00:00Z'
} as Proxy

describe('ProxySelector', () => {
  it('uses a privacy label instead of rendering an empty public endpoint', async () => {
    const wrapper = mount(ProxySelector, {
      props: {
        modelValue: hiddenPublicProxy.id,
        proxies: [hiddenPublicProxy]
      },
      global: {
        stubs: {
          Icon: true,
          Transition: false
        }
      }
    })

    expect(wrapper.get('.select-trigger').text()).toContain('Public proxy hidden')
    expect(wrapper.get('.select-trigger').text()).not.toContain('http://:')

    await wrapper.get('.select-trigger').trigger('click')

    expect(wrapper.get('.select-options').text()).toContain('Public proxy hidden')
    expect(wrapper.get('.select-options').text()).not.toContain('http://:')
  })
})
