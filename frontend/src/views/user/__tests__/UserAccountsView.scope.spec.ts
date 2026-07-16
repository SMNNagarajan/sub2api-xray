import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(process.cwd(), 'src/views/user/UserAccountsView.vue'), 'utf8')

describe('UserAccountsView user scope and admin-equivalent experience', () => {
  it('uses the account management table experience without admin APIs', () => {
    expect(source).toContain('<TablePageLayout>')
    expect(source).toContain('<DataTable')
    expect(source).toContain("t('admin.accounts.columns.todayStats')")
    expect(source).toContain("t('admin.accounts.columns.usageWindows')")
    expect(source).toContain('myResourcesApi.accounts.list')
    expect(source).not.toContain('adminAPI')
    expect(source).not.toContain("'/admin/")
  })

  it('keeps account dialogs within the viewport and scrolls their content', () => {
    expect(source).toContain('max-h-[calc(100dvh-1rem)]')
    expect(source).toContain('overflow-y-auto overscroll-contain')
  })

  it('keeps account filters full width on mobile', () => {
    expect(source).toContain('w-full min-w-0 basis-full')
    expect(source).toContain('lg:w-auto lg:basis-auto lg:flex-1')
  })

  it('supports the expected user account data and batch operations', () => {
    for (const operation of ['export', 'import', 'importCodexSessions', 'importCodexPAT', 'batchUpdate', 'test', 'refresh', 'clearError', 'setSchedulable']) {
      expect(source).toContain(`myResourcesApi.accounts.${operation}`)
    }
  })

  it('does not resend redacted credentials and requires complete replacement secrets', () => {
    expect(source).toContain('const shouldSend = !editingId.value || credentialsTouched.value')
    expect(source).toContain('if (!shouldSend) return undefined')
    expect(source).toContain("if (form.type === 'apikey')")
    expect(source).toContain("mr('messages.apiKeyRequired')")
    expect(source).toContain("if (form.type === 'service_account')")
    expect(source).toContain("mr('messages.serviceAccountRequired')")
  })

  it('keeps visible copy and operation messages localized', () => {
    expect(source).not.toMatch(/[\p{Script=Han}]/u)
    expect(source).toContain("mr('actions.clearError')")
    expect(source).toContain(":title=\"t('common.copy')\"")
  })
})
