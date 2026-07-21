import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import GroupSelector from '../GroupSelector.vue'
import type { AdminGroup } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const group = (id: number, ownerUserId: number | null, name: string) => ({
  id,
  owner_user_id: ownerUserId,
  name,
  description: null,
  platform: 'openai',
  rate_multiplier: 1,
  status: 'active',
  subscription_type: 'standard',
  account_count: 0,
} as AdminGroup)

const groups = [
  group(1, null, 'System group'),
  group(2, 7, 'User 7 group'),
  group(3, 8, 'User 8 group'),
]

const mountSelector = (ownerUserId: number | null) => mount(GroupSelector, {
  props: {
    modelValue: [],
    groups,
    platform: 'openai',
    enforceOwner: true,
    ownerUserId,
  },
  global: {
    stubs: {
      GroupBadge: {
        props: ['name'],
        template: '<span>{{ name }}</span>',
      },
      Icon: true,
    },
  },
})

describe('GroupSelector owner scope', () => {
  it('only exposes system groups for a system account', () => {
    const text = mountSelector(null).text()
    expect(text).toContain('System group')
    expect(text).not.toContain('User 7 group')
    expect(text).not.toContain('User 8 group')
  })

  it('only exposes groups owned by the target user', () => {
    const text = mountSelector(7).text()
    expect(text).toContain('User 7 group')
    expect(text).not.toContain('System group')
    expect(text).not.toContain('User 8 group')
  })
})
