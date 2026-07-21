import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const {
  adminAccountSyncMock,
  adminPreviewSyncMock,
  userAccountSyncMock,
  userPreviewSyncMock
} = vi.hoisted(() => ({
  adminAccountSyncMock: vi.fn(),
  adminPreviewSyncMock: vi.fn(),
  userAccountSyncMock: vi.fn(),
  userPreviewSyncMock: vi.fn()
}))

vi.mock('@/api/admin/accounts', () => ({
  accountsAPI: {
    syncUpstreamModels: adminAccountSyncMock,
    syncUpstreamModelsPreview: adminPreviewSyncMock
  },
  getAntigravityDefaultModelMapping: vi.fn()
}))

vi.mock('@/api/myResources', () => ({
  myResourcesApi: {
    accounts: {
      syncUpstreamModels: userAccountSyncMock,
      syncUpstreamModelsPreview: userPreviewSyncMock,
      getAntigravityDefaultModelMapping: vi.fn()
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showInfo: vi.fn(),
    showSuccess: vi.fn(),
    showError: vi.fn()
  })
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

import ModelWhitelistSelector from '../ModelWhitelistSelector.vue'

function syncButton(wrapper: ReturnType<typeof mount>) {
  const button = wrapper.findAll('button').find(candidate => candidate.text() === 'admin.accounts.syncUpstreamModels')
  if (!button) throw new Error('sync upstream models button not found')
  return button
}

describe('ModelWhitelistSelector scoped upstream sync', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    adminAccountSyncMock.mockResolvedValue({ models: ['admin-model'] })
    adminPreviewSyncMock.mockResolvedValue({ models: ['admin-preview'] })
    userAccountSyncMock.mockResolvedValue({ models: ['user-model'] })
    userPreviewSyncMock.mockResolvedValue({ models: ['user-preview'] })
  })

  it('keeps the existing admin account sync as the default', async () => {
    const wrapper = mount(ModelWhitelistSelector, {
      props: { modelValue: [], platform: 'openai', accountId: 7 },
      global: { stubs: { ModelIcon: true, Icon: true } }
    })

    await syncButton(wrapper).trigger('click')
    await flushPromises()

    expect(adminAccountSyncMock).toHaveBeenCalledWith(7)
    expect(userAccountSyncMock).not.toHaveBeenCalled()
  })

  it('uses only the user account sync API in user scope', async () => {
    const wrapper = mount(ModelWhitelistSelector, {
      props: { modelValue: [], platform: 'openai', accountId: 8, scope: 'user' },
      global: { stubs: { ModelIcon: true, Icon: true } }
    })

    await syncButton(wrapper).trigger('click')
    await flushPromises()

    expect(userAccountSyncMock).toHaveBeenCalledWith(8)
    expect(adminAccountSyncMock).not.toHaveBeenCalled()
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual(['user-model'])
  })

  it('uses only the user preview API for temporary credentials', async () => {
    const credentials = { platform: 'openai', type: 'apikey', base_url: 'https://api.example.com', api_key: 'temporary-key' }
    const wrapper = mount(ModelWhitelistSelector, {
      props: { modelValue: [], platform: 'openai', syncCredentials: credentials, scope: 'user' },
      global: { stubs: { ModelIcon: true, Icon: true } }
    })

    await syncButton(wrapper).trigger('click')
    await flushPromises()

    expect(userPreviewSyncMock).toHaveBeenCalledWith(credentials)
    expect(adminPreviewSyncMock).not.toHaveBeenCalled()
  })

  it('prefers an injected scoped callback over built-in APIs', async () => {
    const callback = vi.fn().mockResolvedValue({ models: ['callback-model'] })
    const wrapper = mount(ModelWhitelistSelector, {
      props: {
        modelValue: [],
        platform: 'openai',
        accountId: 9,
        scope: 'user',
        syncAccountModels: callback
      },
      global: { stubs: { ModelIcon: true, Icon: true } }
    })

    await syncButton(wrapper).trigger('click')
    await flushPromises()

    expect(callback).toHaveBeenCalledWith(9)
    expect(userAccountSyncMock).not.toHaveBeenCalled()
    expect(adminAccountSyncMock).not.toHaveBeenCalled()
  })
})
