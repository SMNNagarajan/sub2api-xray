import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import MyProxyEditorDialog from '../MyProxyEditorDialog.vue'

const api = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  importNodes: vi.fn(),
  sourceCreate: vi.fn(),
  sourceSync: vi.fn(),
}))

vi.mock('@/api/myResources', () => ({
  myResourcesApi: {
    proxies: {
      list: api.list,
      create: api.create,
      update: api.update,
      importNodes: api.importNodes,
      sources: { create: api.sourceCreate, sync: api.sourceSync },
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }),
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

function mountDialog(props: Record<string, unknown> = {}) {
  return mount(MyProxyEditorDialog, {
    props: { show: true, ...props },
    global: {
      stubs: {
        teleport: true,
        transition: false,
      },
    },
  })
}

describe('MyProxyEditorDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.list.mockResolvedValue({ items: [] })
    api.create.mockResolvedValue({ id: 1 })
    api.update.mockResolvedValue({ id: 1 })
    api.importNodes.mockResolvedValue({ created: [], errors: [] })
  })

  it('uses a scrollable BaseDialog and offers all supported creation modes', async () => {
    const wrapper = mountDialog()
    await vi.waitFor(() => expect(api.list).toHaveBeenCalled())

    expect(wrapper.findAll('[data-test^="proxy-create-mode-"]')).toHaveLength(2)
    expect(wrapper.text()).toContain('myResources.proxyEditor.standardCreate')
    expect(wrapper.text()).toContain('myResources.proxyEditor.batchCreate')
  })

  it('shows live batch validity and duplicate statistics', async () => {
    const wrapper = mountDialog({ initialMode: 'batch' })
    const textarea = wrapper.get('[data-test="proxy-batch-input"]')
    await textarea.setValue([
      'socks5://127.0.0.1:1080',
      'socks5://127.0.0.1:1080',
      'not-a-proxy',
      'vless://uuid@example.com:443',
    ].join('\n'))

    expect(wrapper.get('[data-test="proxy-stat-total"]').text()).toContain('4')
    expect(wrapper.get('[data-test="proxy-stat-valid"]').text()).toContain('2')
    expect(wrapper.get('[data-test="proxy-stat-invalid"]').text()).toContain('1')
    expect(wrapper.get('[data-test="proxy-stat-duplicate"]').text()).toContain('1')
  })

  it('does not expose or submit public visibility for user-owned proxies', async () => {
    const wrapper = mountDialog()
    await wrapper.get('[data-test="proxy-name"]').setValue('private-proxy')
    await wrapper.get('[data-test="proxy-host"]').setValue('proxy.example.com')
    await wrapper.get('form').trigger('submit')

    await vi.waitFor(() => expect(api.create).toHaveBeenCalled())
    expect(api.create.mock.calls[0][0]).not.toHaveProperty('is_public')
    expect(wrapper.html()).not.toContain('is_public')
  })

  it('preserves redacted credentials while editing unless fields are changed', async () => {
    const wrapper = mountDialog({
      proxy: {
        id: 7,
        owner_user_id: 9,
        name: 'owned-proxy',
        kind: 'standard',
        protocol: 'socks5',
        host: 'proxy.example.com',
        port: 1080,
        status: 'active',
      },
    })
    await wrapper.get('form').trigger('submit')

    await vi.waitFor(() => expect(api.update).toHaveBeenCalled())
    const payload = api.update.mock.calls[0][1]
    expect(payload).not.toHaveProperty('username')
    expect(payload).not.toHaveProperty('password')
    expect(payload).not.toHaveProperty('extra')
  })
})
