import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(process.cwd(), 'src/views/admin/ProxiesView.vue'), 'utf8')

describe('admin proxy modern mode support', () => {
  it('supports standard and Xray proxy modes without dropping node metadata', () => {
    expect(source).toContain("kind: 'standard' as ProxyKind")
    expect(source).toContain("{ value: 'vless', label: 'VLESS' }")
    expect(source).toContain("{ ...(editingProxy.value.extra || {}), raw: editForm.xray_raw.trim() }")
    expect(source).toContain("proxy.kind || 'standard'")
  })

  it('masks both proxy usernames and passwords until explicitly revealed', () => {
    expect(source).toContain("visiblePasswordIds.has(row.id) ? row.username : '••••••'")
    expect(source).toContain("visiblePasswordIds.has(row.id) ? row.password : '••••••'")
    expect(source).toContain("t('admin.proxies.showCredentials')")
  })
})
