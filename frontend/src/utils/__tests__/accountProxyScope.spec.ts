import { describe, expect, it } from 'vitest'

import type { Proxy } from '@/types'
import {
  filterAccountCompatibleProxies,
  toUnixSeconds,
  toUserAccountPayload,
} from '@/utils/accountProxyScope'

const proxy = (id: number, owner_user_id: number | null, is_public = false) => ({
  id,
  owner_user_id,
  is_public,
} as Proxy)

describe('account proxy scope', () => {
  const proxies = [
    proxy(1, null),
    proxy(2, null, true),
    proxy(3, 7),
    proxy(4, 8),
  ]

  it('only exposes system proxies to system accounts', () => {
    expect(filterAccountCompatibleProxies(proxies, null).map(item => item.id)).toEqual([1, 2])
  })

  it('only exposes owned and public proxies to user accounts', () => {
    expect(filterAccountCompatibleProxies(proxies, 7).map(item => item.id)).toEqual([2, 3])
  })

  it('normalizes account timestamps for the user API', () => {
    expect(toUnixSeconds('2026-07-19T12:00:00.000Z')).toBe(1784462400)
    expect(toUserAccountPayload({ expires_at: 1784462400, proxy_id: 0 })).toEqual({
      expires_at: '2026-07-19T12:00:00.000Z',
      proxy_id: null,
    })
  })
})
