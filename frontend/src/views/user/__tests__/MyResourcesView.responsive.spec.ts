import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'

const source = readFileSync(resolve(process.cwd(), 'src/views/user/MyResourcesView.vue'), 'utf8')
const proxyEditorSource = readFileSync(resolve(process.cwd(), 'src/components/user/MyProxyEditorDialog.vue'), 'utf8')
const globalStyles = readFileSync(resolve(process.cwd(), 'src/style.css'), 'utf8')

describe('MyResourcesView responsive localization', () => {
  it('delegates responsive tables and mobile cards to the shared DataTable', () => {
    expect(source).toContain('<DataTable')
    expect(source).toContain(':columns="alignedColumns"')
    expect(source).not.toContain('sm:sticky sm:right-0')
    expect(source).not.toContain('<div v-else class="space-y-4">')
  })

  it('aligns every user resource with the administrator table system', () => {
    expect(source).toContain('<TablePageLayout>')
    expect(source).toContain('<DataTable')
    expect(source).toContain(':columns="alignedColumns"')
    expect(source).not.toContain('adminAlignedResource')
    expect(source).not.toContain('<div v-else class="space-y-4">')
    for (const resource of ['groups', 'accounts', 'proxies', 'assigned-subscriptions', 'redeem-codes', 'account-logs', 'upstream-errors']) {
      expect(source).toContain(`'${resource}'`)
    }
    expect(source).toContain("mr('columns.visibility')")
    expect(source).toContain("mr('columns.proxyMode')")
    expect(source).toContain("mr('columns.location')")
    expect(source).toContain("mr('columns.latency')")
    expect(source).toContain("mr('columns.usage')")
    expect(source).toContain("mr('actions.userRates')")
    expect(source).toContain('max-w-[70%] break-all text-xs sm:max-w-none sm:whitespace-nowrap sm:break-normal')
    expect(source).not.toContain('class="code whitespace-nowrap text-xs"')

    const testAction = source.indexOf("mr('actions.testConnection')")
    const qualityAction = source.indexOf("mr('actions.quality')")
    const proxyEditAction = source.indexOf("resource === 'proxies' && canMutateItem(row)")
    expect(testAction).toBeGreaterThan(-1)
    expect(testAction).toBeLessThan(qualityAction)
    expect(qualityAction).toBeLessThan(proxyEditAction)
  })

  it('constrains every resource dialog to the viewport with an internal scroll area', () => {
    const compactConstraints = source.match(/max-h-\[calc\(100dvh-1rem\)\]/g) || []
    const wideConstraints = source.match(/sm:max-h-\[calc\(100dvh-2rem\)\]/g) || []
    expect(compactConstraints.length).toBeGreaterThanOrEqual(9)
    expect(wideConstraints).toHaveLength(compactConstraints.length)
    expect((source.match(/overscroll-contain/g) || []).length).toBeGreaterThanOrEqual(compactConstraints.length)
    expect(source).not.toContain('max-h-[90vh]')
  })

  it('uses localized labels for the toolbar actions found during UI review', () => {
    for (const text of ['>Columns<', '>Export<', '>Import<', '>Test<', '>Refresh<', '>Sources<', '>Stats<']) {
      expect(source).not.toContain(text)
    }
  })

  it('uses project Select controls and proxy-specific filters', () => {
    expect((source.match(/<Select/g) || []).length).toBeGreaterThan(10)
    expect((source.match(/<select[^>]+multiple/g) || []).length).toBeGreaterThanOrEqual(3)
    expect(source).toContain('<MyProxyEditorDialog')
    expect(proxyEditorSource).toContain('<BaseDialog')
    expect(proxyEditorSource).toContain('width="normal"')
    expect(proxyEditorSource).toContain("type CreateMode = 'standard' | 'batch'")
    expect(globalStyles).toContain('max-h-[calc(100dvh-1rem)] sm:max-h-[calc(100dvh-2rem)]')
    expect(globalStyles).toContain('@apply flex-1 overflow-y-auto')
    expect(source).toContain('v-model="filters.type"')
    expect(source).toContain('v-model="filters.protocol"')
    expect(source).toContain("mr(`filters.searchByResource.${resource.value}`)")
    expect(source).not.toContain(':placeholder="mr(\'filters.search\')"')
  })

  it('does not submit redacted proxy credentials unless the user edits them', () => {
    expect(proxyEditorSource).toContain('usernameDirty.value')
    expect(proxyEditorSource).toContain('passwordDirty.value')
    expect(proxyEditorSource).toContain('nodeContentDirty.value')
    expect(proxyEditorSource).toContain('if (!props.proxy || usernameDirty.value)')
    expect(proxyEditorSource).toContain('if (!props.proxy || passwordDirty.value)')
    expect(proxyEditorSource).toContain("if (inputMode.value === 'xray' && nodeContentDirty.value)")
  })

  it('supports repeatable redeem codes through owner-scoped usage details', () => {
    expect(source).toContain('v-model="editorForm.redeem.repeatable"')
    expect(source).toContain('v-model.number="editorForm.redeem.max_uses"')
    expect(source).toContain('myResourcesApi.redeemCodes.usages')
    expect(source).toContain('#cell-usage_count')
  })

  it('renders scoped usage and upstream-error details without a second unscoped request', () => {
    expect(source).toContain("resource.value === 'upstream-errors'")
    expect(source).toContain("mr('details.accountLogTitle')")
    expect(source).toContain("mr('details.upstreamErrorTitle')")
    expect(source).toContain('@click="openRecordDetails(row)"')
    expect(source).not.toContain('usage.recordDetails')
    expect(source).not.toMatch(/[\p{Script=Han}]/u)
  })

  it('defines the user-resource message namespace in both locales', () => {
    for (const messages of [zh, en]) {
      expect(messages.myResources.actions.columns).toEqual(expect.any(String))
      expect(messages.myResources.actions.exportCsv).toEqual(expect.any(String))
      expect(messages.myResources.actions.userRates).toEqual(expect.any(String))
      expect(messages.myResources.actions.testConnection).toEqual(expect.any(String))
      expect(messages.myResources.states.healthy).toEqual(expect.any(String))
      expect(messages.myResources.fields.subscriptionUrl).toEqual(expect.any(String))
      expect(messages.myResources.table.actions).toEqual(expect.any(String))
      expect(messages.myResources.table.qualityInline).toEqual(expect.any(String))
      expect(messages.myResources.messages.oauthUrlMissing).toEqual(expect.any(String))
      expect(messages.myResources.overrides.title).toEqual(expect.any(String))
    }
  })
})
