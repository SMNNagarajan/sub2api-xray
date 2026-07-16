<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap-reverse items-start justify-between gap-3">
          <div class="flex w-full min-w-0 basis-full flex-wrap items-center gap-3 lg:w-auto lg:basis-auto lg:flex-1">
            <SearchInput v-model="params.search" :placeholder="t('admin.accounts.searchAccounts')" class="w-full sm:w-64" @search="reload" />
            <Select v-model="params.platform" class="w-40" :options="platformFilterOptions" @change="reload" />
            <Select v-model="params.type" class="w-40" :options="typeFilterOptions" @change="reload" />
            <Select v-model="params.status" class="w-40" :options="statusFilterOptions" @change="reload" />
            <Select v-model="params.group_id" class="w-44" :options="groupFilterOptions" searchable @change="reload" />
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="reload">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            </button>
            <div class="relative">
              <button class="btn btn-secondary px-2 md:px-3" :title="t('admin.accounts.autoRefresh')" @click="showAutoRefresh = !showAutoRefresh; showTools = false">
                <Icon name="refresh" size="sm" :class="autoRefreshEnabled ? 'animate-spin' : ''" />
                <span class="hidden md:inline">{{ autoRefreshEnabled ? t('admin.accounts.autoRefreshCountdown', { seconds: countdown }) : t('admin.accounts.autoRefresh') }}</span>
              </button>
              <div v-if="showAutoRefresh" class="absolute right-0 z-40 mt-2 w-56 rounded-lg border border-gray-200 bg-white p-2 shadow-lg dark:border-dark-700 dark:bg-dark-800">
                <button class="account-menu-item" @click="autoRefreshEnabled = !autoRefreshEnabled"><span>{{ t('admin.accounts.enableAutoRefresh') }}</span><Icon v-if="autoRefreshEnabled" name="check" size="sm" /></button>
                <button v-for="seconds in [10, 30, 60]" :key="seconds" class="account-menu-item" @click="setAutoRefreshInterval(seconds)">
                  <span>{{ seconds }}s</span><Icon v-if="autoRefreshInterval === seconds" name="check" size="sm" />
                </button>
              </div>
            </div>
            <div class="relative">
              <button class="btn btn-secondary px-2 md:px-3" :title="t('admin.accounts.moreActions')" @click="showTools = !showTools; showAutoRefresh = false">
                <Icon name="more" size="sm" /><span class="hidden md:inline">{{ t('admin.accounts.moreActions') }}</span><Icon name="chevronDown" size="xs" />
              </button>
              <div v-if="showTools" class="absolute right-0 z-40 mt-2 w-[min(20rem,calc(100vw-2rem))] overflow-hidden rounded-lg border border-gray-200 bg-white shadow-xl dark:border-dark-700 dark:bg-dark-800">
                <div class="max-h-[70vh] overflow-y-auto p-2">
                  <button class="account-menu-item" @click="openImport"><Icon name="upload" size="sm" /><span>{{ mr('actions.import') }}</span></button>
                  <button class="account-menu-item" @click="exportAccounts"><Icon name="download" size="sm" /><span>{{ mr('actions.export') }}</span></button>
                  <button class="account-menu-item" @click="openCodexSession"><Icon name="upload" size="sm" /><span>{{ mr('actions.codexSession') }}</span></button>
                  <button class="account-menu-item" @click="openCodexPAT"><Icon name="upload" size="sm" /><span>{{ mr('actions.codexPat') }}</span></button>
                  <div class="my-2 border-t border-gray-100 dark:border-dark-700"></div>
                  <div class="px-3 py-2 text-xs font-semibold uppercase text-gray-400">{{ t('admin.accounts.viewColumns') }}</div>
                  <button v-for="column in toggleableColumns" :key="column.key" class="account-menu-item" @click="toggleColumn(column.key)">
                    <span class="truncate">{{ column.label }}</span><Icon v-if="isColumnVisible(column.key)" name="check" size="sm" />
                  </button>
                </div>
              </div>
            </div>
            <button class="btn btn-primary" @click="openCreate">{{ t('admin.accounts.createAccount') }}</button>
          </div>
        </div>
      </template>

      <template #table>
        <div v-if="selectedIds.length" class="flex flex-wrap items-center justify-between gap-3 border-b border-primary-100 bg-primary-50 px-4 py-3 dark:border-primary-900/40 dark:bg-primary-900/20">
          <span class="text-sm font-medium text-primary-800 dark:text-primary-200">{{ t('admin.accounts.selectedCount', { count: selectedIds.length }) }}</span>
          <div class="flex flex-wrap gap-2">
            <button class="btn btn-sm btn-secondary" @click="batchRefresh">{{ mr('actions.refresh') }}</button>
            <button class="btn btn-sm btn-secondary" @click="batchClearErrors">{{ mr('actions.clearError') }}</button>
            <button class="btn btn-sm btn-secondary" @click="batchOpen = true">{{ mr('actions.batchEdit') }}</button>
            <button class="btn btn-sm btn-secondary" @click="selectedIds = []">{{ t('common.cancel') }}</button>
          </div>
        </div>
        <DataTable :columns="visibleColumns" :data="accounts" :loading="loading" row-key="id" server-side-sort default-sort-key="name" default-sort-order="asc" @sort="handleSort">
          <template #header-select><input type="checkbox" :checked="allSelected" @change="toggleAll" /></template>
          <template #cell-select="{ row }"><input type="checkbox" :checked="selectedIds.includes(row.id)" @change="toggleSelected(row.id)" /></template>
          <template #cell-name="{ row }"><div class="min-w-36"><div class="font-medium text-gray-900 dark:text-white">{{ row.name }}</div><div v-if="credentialEmail(row)" class="max-w-48 truncate text-xs text-gray-500">{{ credentialEmail(row) }}</div></div></template>
          <template #cell-platform_type="{ row }"><PlatformTypeBadge :platform="row.platform" :type="row.type" :plan-type="row.credentials?.plan_type" :privacy-mode="row.extra?.privacy_mode" :subscription-expires-at="row.credentials?.subscription_expires_at" /></template>
          <template #cell-capacity="{ row }"><AccountCapacityCell :account="row" /></template>
          <template #cell-status="{ row }"><AccountStatusIndicator :account="row" /></template>
          <template #cell-schedulable="{ row }"><button class="relative h-6 w-11 rounded-full transition-colors" :class="row.schedulable ? 'bg-primary-600' : 'bg-gray-300 dark:bg-dark-600'" :title="t('admin.accounts.columns.schedulable')" @click="toggleSchedulable(row)"><span class="absolute top-0.5 h-5 w-5 rounded-full bg-white shadow transition-transform" :class="row.schedulable ? 'left-5' : 'left-0.5'"></span></button></template>
          <template #cell-today_stats="{ row }"><span class="font-medium text-gray-900 dark:text-white">{{ row.today_request_count || 0 }}</span><span class="ml-1 text-xs text-gray-500"> req</span></template>
          <template #cell-groups="{ row }"><div class="flex max-w-56 flex-wrap gap-1"><span v-for="group in row.groups || []" :key="group.id" class="badge badge-gray">{{ group.name }}</span><span v-if="!(row.groups || []).length">-</span></div></template>
          <template #cell-usage="{ row }"><div class="text-xs text-gray-500"><div>{{ row.session_window_status || '-' }}</div><div v-if="row.rate_limit_reset_at">{{ formatDate(row.rate_limit_reset_at) }}</div></div></template>
          <template #cell-proxy="{ row }"><div class="max-w-44"><div class="truncate">{{ row.proxy_name || '-' }}</div><div v-if="row.proxy_protocol" class="text-xs uppercase text-gray-500">{{ row.proxy_protocol }}</div></div></template>
          <template #cell-priority="{ row }">{{ row.priority ?? 0 }}</template>
          <template #cell-rate_multiplier="{ row }">{{ Number(row.rate_multiplier ?? 1).toFixed(2) }}x</template>
          <template #cell-last_used_at="{ row }">{{ formatDate(row.last_used_at) }}</template>
          <template #cell-created_at="{ row }">{{ formatDate(row.created_at) }}</template>
          <template #cell-expires_at="{ row }">{{ formatDate(row.expires_at) }}</template>
          <template #cell-notes="{ row }"><span class="block max-w-48 truncate" :title="row.notes">{{ row.notes || '-' }}</span></template>
          <template #cell-actions="{ row }">
            <div class="flex items-center justify-end gap-1">
              <button class="icon-action" :title="mr('actions.test')" @click="testAccount(row)"><Icon name="play" size="sm" /></button>
              <button class="icon-action" :title="mr('actions.refresh')" @click="refreshAccount(row)"><Icon name="refresh" size="sm" /></button>
              <button class="icon-action" :title="t('common.edit')" @click="openEdit(row)"><Icon name="edit" size="sm" /></button>
              <button class="icon-action text-red-500" :title="t('common.delete')" @click="deleteAccount(row)"><Icon name="trash" size="sm" /></button>
            </div>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination v-if="pagination.total" :page="pagination.page" :total="pagination.total" :page-size="pagination.page_size" @update:page="changePage" @update:page-size="changePageSize" />
      </template>
    </TablePageLayout>

    <div v-if="editorOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-2 sm:p-4">
      <form class="flex max-h-[calc(100dvh-1rem)] w-full max-w-3xl flex-col overflow-hidden rounded-lg bg-white shadow-xl dark:bg-dark-800 sm:max-h-[calc(100dvh-2rem)]" @submit.prevent="saveAccount">
        <div class="flex shrink-0 items-center justify-between border-b border-gray-200 p-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ editingId ? t('admin.accounts.editAccount') : t('admin.accounts.createAccount') }}</h2>
          <button class="btn btn-sm btn-secondary" type="button" @click="editorOpen = false">{{ t('common.close') }}</button>
        </div>
        <div class="min-h-0 flex-1 space-y-4 overflow-y-auto overscroll-contain p-4">
          <div class="grid gap-3 md:grid-cols-2">
            <label class="field"><span>{{ mr('fields.name') }}</span><input v-model.trim="form.name" class="input" required /></label>
            <label class="field"><span>{{ mr('fields.platform') }}</span><Select v-model="form.platform" :options="platformOptions" /></label>
            <label class="field"><span>{{ mr('fields.type') }}</span><Select v-model="form.type" :options="accountTypeOptions" /></label>
            <label class="field"><span>{{ mr('fields.status') }}</span><Select v-model="form.status" :options="accountStatusOptions" /></label>
            <label class="field md:col-span-2"><span>{{ mr('fields.groups') }}</span><select v-model="form.group_ids" class="input min-h-24" multiple><option v-for="group in compatibleGroups" :key="group.id" :value="Number(group.id)">{{ group.name }}</option></select></label>
            <label class="field"><span>{{ mr('fields.proxy') }}</span><Select v-model="form.proxy_id" :options="proxySelectOptions" searchable /></label>
            <section v-if="['oauth', 'setup-token'].includes(form.type)" class="space-y-3 rounded-md border border-gray-200 p-3 md:col-span-2 dark:border-dark-700">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <div>
                  <div class="text-sm font-medium text-gray-900 dark:text-white">OAuth / Setup Token</div>
                  <div class="text-xs text-gray-500">{{ form.platform }} · {{ form.type }}</div>
                </div>
                <button type="button" class="btn btn-secondary" :disabled="oauth.loading" @click="generateOAuthURL">{{ mr('actions.generateAuthUrl') }}</button>
              </div>
              <div v-if="form.platform === 'gemini'" class="grid gap-3 md:grid-cols-3">
                <label class="field"><span>{{ mr('fields.oauthType') }}</span><Select v-model="oauth.oauth_type" :options="geminiOAuthOptions" /></label>
                <label class="field"><span>{{ mr('fields.projectId') }}</span><input v-model.trim="oauth.project_id" class="input" /></label>
                <label class="field"><span>{{ mr('fields.tierId') }}</span><input v-model.trim="oauth.tier_id" class="input" /></label>
              </div>
              <div v-if="oauth.auth_url" class="flex min-w-0 items-center gap-2 rounded-md bg-gray-50 p-2 dark:bg-dark-900">
                <a class="min-w-0 flex-1 truncate text-sm text-primary-600 hover:underline" :href="oauth.auth_url" target="_blank" rel="noopener noreferrer">{{ oauth.auth_url }}</a>
                <button type="button" class="icon-action shrink-0" :title="t('common.copy')" @click="copyOAuthURL"><Icon name="copy" size="sm" /></button>
              </div>
              <div v-if="oauth.session_id" class="grid gap-2 md:grid-cols-[1fr_auto] md:items-end">
                <label class="field"><span>{{ mr('fields.callbackOrCode') }}</span><textarea v-model.trim="oauth.callback" class="input min-h-20 font-mono text-xs" spellcheck="false"></textarea></label>
                <button type="button" class="btn btn-secondary" :disabled="oauth.loading || !oauth.callback.trim()" @click="exchangeOAuthCode">{{ mr('actions.completeAuthorization') }}</button>
              </div>
              <div v-if="form.platform === 'anthropic'" class="grid gap-2 md:grid-cols-[1fr_auto] md:items-end">
                <label class="field"><span>{{ mr('fields.sessionKey') }}</span><textarea v-model.trim="oauth.session_key" class="input min-h-20 font-mono text-xs" spellcheck="false"></textarea></label>
                <button type="button" class="btn btn-secondary" :disabled="oauth.loading || !oauth.session_key.trim()" @click="exchangeOAuthCookie">{{ mr('actions.sessionKeyAuthorization') }}</button>
              </div>
              <div v-if="oauth.error" class="text-sm text-red-600 dark:text-red-300">{{ oauth.error }}</div>
            </section>
            <section v-if="form.type === 'apikey'" class="grid gap-3 rounded-md border border-gray-200 p-3 md:col-span-2 md:grid-cols-2 dark:border-dark-700">
              <label class="field"><span>{{ mr('fields.apiKey') }}</span><input v-model="credentialFields.api_key" type="password" class="input font-mono text-xs" autocomplete="new-password" :placeholder="editingId ? mr('fields.leaveBlankToKeep') : ''" @input="credentialsTouched = true" /></label>
              <label class="field"><span>{{ mr('fields.baseUrl') }}</span><input v-model.trim="credentialFields.base_url" class="input font-mono text-xs" placeholder="https://api.example.com/v1" @input="credentialsTouched = true" /></label>
            </section>
            <section v-if="form.type === 'service_account'" class="rounded-md border border-gray-200 p-3 md:col-span-2 dark:border-dark-700">
              <label class="field"><span>{{ mr('fields.serviceAccountJson') }}</span><textarea v-model="credentialFields.service_account_json" class="input min-h-40 font-mono text-xs" spellcheck="false" :placeholder="editingId ? mr('fields.leaveBlankToKeep') : ''" @input="credentialsTouched = true"></textarea></label>
            </section>
            <label class="field"><span>{{ mr('fields.priority') }}</span><input v-model.number="form.priority" type="number" class="input" /></label>
            <label class="field"><span>{{ mr('fields.concurrency') }}</span><input v-model.number="form.concurrency" type="number" min="0" class="input" /></label>
            <label class="field"><span>{{ mr('fields.loadFactor') }}</span><input v-model.number="form.load_factor" type="number" min="0" step="0.1" class="input" /></label>
            <label class="field"><span>{{ mr('fields.rateMultiplier') }}</span><input v-model.number="form.rate_multiplier" type="number" min="0" step="0.01" class="input" /></label>
            <label class="field"><span>{{ mr('fields.expiresAt') }}</span><input v-model="form.expires_at" type="datetime-local" class="input" /></label>
            <label class="flex items-center gap-2 rounded-md border border-gray-200 p-3 dark:border-dark-700"><input v-model="form.schedulable" type="checkbox" /><span>{{ mr('fields.schedulable') }}</span></label>
            <div class="md:col-span-2">
              <button type="button" class="inline-flex items-center gap-2 text-sm font-medium text-gray-600 hover:text-primary-600 dark:text-gray-300" @click="showAdvanced = !showAdvanced">
                <Icon name="cog" size="sm" />{{ mr('fields.advancedSettings') }}<Icon :name="showAdvanced ? 'chevronUp' : 'chevronDown'" size="xs" />
              </button>
              <div v-if="showAdvanced" class="mt-3 grid gap-3 rounded-md border border-gray-200 p-3 dark:border-dark-700">
                <label class="field"><span>{{ mr('fields.credentials') }}</span><textarea v-model="form.credentials_text" class="input min-h-28 font-mono text-xs" spellcheck="false" @input="credentialsTouched = true"></textarea></label>
                <label class="field"><span>{{ mr('fields.extraJson') }}</span><textarea v-model="form.extra_text" class="input min-h-20 font-mono text-xs" spellcheck="false"></textarea></label>
              </div>
            </div>
            <label class="field md:col-span-2"><span>{{ mr('fields.notes') }}</span><textarea v-model="form.notes" class="input min-h-20"></textarea></label>
          </div>
          <div v-if="editorError" class="rounded-md bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/30 dark:text-red-200">{{ editorError }}</div>
        </div>
        <div class="flex shrink-0 justify-end gap-2 border-t border-gray-200 p-4 dark:border-dark-700"><button class="btn btn-secondary" type="button" @click="editorOpen = false">{{ t('common.cancel') }}</button><button class="btn btn-primary" :disabled="saving">{{ t('common.save') }}</button></div>
      </form>
    </div>

    <div v-if="textDialogOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-2 sm:p-4">
      <form class="flex max-h-[calc(100dvh-1rem)] w-full max-w-2xl flex-col overflow-hidden rounded-lg bg-white shadow-xl dark:bg-dark-800" @submit.prevent="submitTextDialog">
        <div class="flex items-center justify-between border-b border-gray-200 p-4 dark:border-dark-700"><h2 class="font-semibold text-gray-900 dark:text-white">{{ textDialogTitle }}</h2><button type="button" class="btn btn-sm btn-secondary" @click="textDialogOpen = false">{{ t('common.close') }}</button></div>
        <div class="min-h-0 flex-1 space-y-3 overflow-y-auto p-4"><textarea v-model="textDialogValue" class="input min-h-72 font-mono text-xs" spellcheck="false"></textarea><div v-if="editorError" class="text-sm text-red-600">{{ editorError }}</div></div>
        <div class="flex justify-end gap-2 border-t border-gray-200 p-4 dark:border-dark-700"><button class="btn btn-secondary" type="button" @click="textDialogOpen = false">{{ t('common.cancel') }}</button><button class="btn btn-primary" :disabled="saving">{{ t('common.save') }}</button></div>
      </form>
    </div>

    <div v-if="batchOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-2 sm:p-4">
      <form class="flex w-full max-w-lg flex-col overflow-hidden rounded-lg bg-white shadow-xl dark:bg-dark-800" @submit.prevent="submitBatchEdit">
        <div class="border-b border-gray-200 p-4 font-semibold dark:border-dark-700">{{ mr('batch.editAccounts') }}</div>
        <div class="space-y-3 p-4"><label class="field"><span>{{ mr('fields.status') }}</span><Select v-model="batchForm.status" :options="accountStatusOptions" /></label><label class="field"><span>{{ mr('fields.priority') }}</span><input v-model.number="batchForm.priority" class="input" type="number" /></label><label class="field"><span>{{ mr('fields.notes') }}</span><textarea v-model="batchForm.notes" class="input min-h-20"></textarea></label></div>
        <div class="flex justify-end gap-2 border-t border-gray-200 p-4 dark:border-dark-700"><button class="btn btn-secondary" type="button" @click="batchOpen = false">{{ t('common.cancel') }}</button><button class="btn btn-primary">{{ mr('actions.apply') }}</button></div>
      </form>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import AccountCapacityCell from '@/components/account/AccountCapacityCell.vue'
import AccountStatusIndicator from '@/components/account/AccountStatusIndicator.vue'
import type { Column } from '@/components/common/types'
import { myResourcesApi, type ResourceItem } from '@/api/myResources'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { getUserAccountTypeOptions, USER_ACCOUNT_STATUS_OPTIONS, USER_ACCOUNT_TYPE_OPTIONS } from '@/utils/userResourceOptions'

const { t } = useI18n()
const mr = (key: string, values?: Record<string, unknown>) => t(`myResources.${key}`, values || {})
const appStore = useAppStore()
const loading = ref(false)
const saving = ref(false)
const accounts = ref<ResourceItem[]>([])
const groups = ref<ResourceItem[]>([])
const proxies = ref<ResourceItem[]>([])
const selectedIds = ref<number[]>([])
const params = reactive({ search: '', platform: '', type: '', status: '', group_id: '' as string | number, sort_by: 'name', sort_order: 'asc' })
const pagination = reactive({ page: 1, page_size: 20, total: 0, pages: 1 })
const hiddenColumns = ref(new Set<string>())
const showTools = ref(false)
const showAutoRefresh = ref(false)
const autoRefreshEnabled = ref(false)
const autoRefreshInterval = ref(30)
const countdown = ref(30)
let timer: ReturnType<typeof setInterval> | undefined

const allColumns = computed<Column[]>(() => [
  { key: 'select', label: '' }, { key: 'name', label: t('admin.accounts.columns.name'), sortable: true },
  { key: 'platform_type', label: t('admin.accounts.columns.platformType') }, { key: 'capacity', label: t('admin.accounts.columns.capacity') },
  { key: 'status', label: t('admin.accounts.columns.status'), sortable: true }, { key: 'schedulable', label: t('admin.accounts.columns.schedulable'), sortable: true },
  { key: 'today_stats', label: t('admin.accounts.columns.todayStats') }, { key: 'groups', label: t('admin.accounts.columns.groups') },
  { key: 'usage', label: t('admin.accounts.columns.usageWindows') }, { key: 'proxy', label: t('admin.accounts.columns.proxy') },
  { key: 'priority', label: t('admin.accounts.columns.priority'), sortable: true }, { key: 'rate_multiplier', label: t('admin.accounts.columns.billingRateMultiplier'), sortable: true },
  { key: 'last_used_at', label: t('admin.accounts.columns.lastUsed'), sortable: true }, { key: 'created_at', label: t('admin.accounts.columns.createdAt'), sortable: true },
  { key: 'expires_at', label: t('admin.accounts.columns.expiresAt') }, { key: 'notes', label: t('admin.accounts.columns.notes') }, { key: 'actions', label: t('admin.accounts.columns.actions') },
])
const visibleColumns = computed(() => allColumns.value.filter(column => !hiddenColumns.value.has(column.key)))
const toggleableColumns = computed(() => allColumns.value.filter(column => !['select', 'actions'].includes(column.key)))
const allSelected = computed(() => accounts.value.length > 0 && accounts.value.every(row => selectedIds.value.includes(Number(row.id))))
const platformOptions: SelectOption[] = ['anthropic', 'openai', 'gemini', 'antigravity', 'grok'].map(value => ({ value, label: value === 'anthropic' ? 'Anthropic' : value === 'openai' ? 'OpenAI' : value[0].toUpperCase() + value.slice(1) }))
const platformFilterOptions = computed(() => [{ value: '', label: t('admin.accounts.allPlatforms') }, ...platformOptions])
const typeFilterOptions = computed(() => [{ value: '', label: t('admin.accounts.allTypes') }, ...USER_ACCOUNT_TYPE_OPTIONS.map(option => ({ value: option.value, label: accountTypeLabel(option.value) }))])
const statusFilterOptions = computed(() => [{ value: '', label: t('admin.accounts.allStatus') }, ...USER_ACCOUNT_STATUS_OPTIONS.map(option => ({ value: option.value, label: statusLabel(option.value) }))])
const groupFilterOptions = computed(() => [{ value: '', label: t('admin.accounts.allGroups') }, ...groups.value.map(group => ({ value: String(group.id), label: String(group.name) }))])
const accountTypeOptions = computed(() => getUserAccountTypeOptions(form.platform).map(option => ({ value: option.value, label: accountTypeLabel(option.value) })))
const accountStatusOptions = computed(() => USER_ACCOUNT_STATUS_OPTIONS.map(option => ({ value: option.value, label: statusLabel(option.value) })))
const compatibleGroups = computed(() => groups.value.filter(group => group.platform === form.platform))
const proxySelectOptions = computed(() => [{ value: 0, label: mr('fields.noProxy') }, ...proxies.value.map(proxy => ({ value: Number(proxy.id), label: `${proxy.name} · ${proxy.protocol}` }))])

const editorOpen = ref(false)
const editingId = ref<number | null>(null)
const editorError = ref('')
const form = reactive({ name: '', platform: 'anthropic', type: 'oauth', status: 'active', schedulable: true, group_ids: [] as number[], proxy_id: 0, priority: 50, concurrency: 3, load_factor: 1, rate_multiplier: 1, expires_at: '', credentials_text: '{}', extra_text: '{}', notes: '' })
const credentialFields = reactive({ api_key: '', base_url: '', service_account_json: '' })
const credentialsTouched = ref(false)
const showAdvanced = ref(false)
const oauth = reactive({ auth_url: '', session_id: '', state: '', callback: '', session_key: '', project_id: '', oauth_type: 'code_assist', tier_id: '', loading: false, error: '' })
const geminiOAuthOptions: SelectOption[] = [{ value: 'code_assist', label: 'Code Assist' }, { value: 'antigravity', label: 'Antigravity' }]
const textDialogOpen = ref(false)
const textDialogMode = ref<'import' | 'codex-session' | 'codex-pat'>('import')
const textDialogValue = ref('')
const textDialogTitle = computed(() => textDialogMode.value === 'import' ? mr('actions.import') : textDialogMode.value === 'codex-session' ? mr('actions.codexSession') : mr('actions.codexPat'))
const batchOpen = ref(false)
const batchForm = reactive({ status: 'active', priority: 50, notes: '' })

async function loadReferences() {
  const [groupPage, proxyPage] = await Promise.all([myResourcesApi.groups.list({ page: 1, page_size: 1000 }), myResourcesApi.proxies.list({ page: 1, page_size: 1000 })])
  groups.value = groupPage.items
  proxies.value = proxyPage.items
}
async function reload() {
  loading.value = true
  try {
    const result = await myResourcesApi.accounts.list({ ...params, group_id: params.group_id || undefined, page: pagination.page, page_size: pagination.page_size })
    accounts.value = result.items
    Object.assign(pagination, { total: result.total, pages: result.pages })
    selectedIds.value = selectedIds.value.filter(id => accounts.value.some(row => Number(row.id) === id))
  } catch (error) { appStore.showError(extractApiErrorMessage(error, mr('messages.loadAccountFailed'))) } finally { loading.value = false }
}
function resetOAuth() { Object.assign(oauth, { auth_url: '', session_id: '', state: '', callback: '', session_key: '', project_id: '', oauth_type: 'code_assist', tier_id: '', loading: false, error: '' }) }
function resetCredentialFields(credentials: ResourceItem = {}) {
  Object.assign(credentialFields, {
    api_key: String(credentials.api_key || ''),
    base_url: String(credentials.base_url || ''),
    service_account_json: String(credentials.service_account_json || ''),
  })
  credentialsTouched.value = false
  showAdvanced.value = false
}
function openCreate() {
  editingId.value = null
  Object.assign(form, { name: '', platform: 'anthropic', type: 'oauth', status: 'active', schedulable: true, group_ids: [], proxy_id: 0, priority: 50, concurrency: 3, load_factor: 1, rate_multiplier: 1, expires_at: '', credentials_text: '{}', extra_text: '{}', notes: '' })
  resetCredentialFields()
  resetOAuth()
  editorError.value = ''
  editorOpen.value = true
}
async function openEdit(row: ResourceItem) {
  editorError.value = ''
  try {
    const item = await myResourcesApi.accounts.get(Number(row.id))
    const credentials = item.credentials_redacted ? {} : (item.credentials || {})
    editingId.value = Number(item.id)
    Object.assign(form, { name: item.name || '', platform: item.platform || 'anthropic', type: item.type || 'oauth', status: item.status || 'active', schedulable: item.schedulable !== false, group_ids: (item.groups || []).map((group: ResourceItem) => Number(group.id)), proxy_id: Number(item.proxy_id || 0), priority: Number(item.priority || 0), concurrency: Number(item.concurrency || 0), load_factor: Number(item.load_factor || 1), rate_multiplier: Number(item.rate_multiplier || 1), expires_at: toDateTimeLocal(item.expires_at), credentials_text: item.credentials_redacted ? '' : JSON.stringify(credentials, null, 2), extra_text: JSON.stringify(item.extra || {}, null, 2), notes: item.notes || '' })
    resetCredentialFields(credentials)
    resetOAuth()
    editorOpen.value = true
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, mr('messages.loadAccountFailed')))
  }
}
function buildCredentialPayload(): ResourceItem | undefined {
  const shouldSend = !editingId.value || credentialsTouched.value
  if (!shouldSend) return undefined
  const credentials = form.credentials_text.trim() ? JSON.parse(form.credentials_text) : {}
  if (form.type === 'apikey') {
    if (!credentialFields.api_key.trim()) throw new Error(mr('messages.apiKeyRequired'))
    credentials.api_key = credentialFields.api_key.trim()
    if (credentialFields.base_url.trim()) credentials.base_url = credentialFields.base_url.trim()
    else delete credentials.base_url
  }
  if (form.type === 'service_account') {
    if (!credentialFields.service_account_json.trim()) throw new Error(mr('messages.serviceAccountRequired'))
    credentials.service_account_json = credentialFields.service_account_json.trim()
  }
  return credentials
}
async function saveAccount() {
  saving.value = true
  editorError.value = ''
  try {
    const payload: ResourceItem = { name: form.name, platform: form.platform, type: form.type, status: form.status, schedulable: form.schedulable, group_ids: form.group_ids, proxy_id: form.proxy_id || null, priority: Number(form.priority), concurrency: Number(form.concurrency), load_factor: Number(form.load_factor), rate_multiplier: Number(form.rate_multiplier), expires_at: form.expires_at ? new Date(form.expires_at).toISOString() : null, extra: JSON.parse(form.extra_text || '{}'), notes: form.notes }
    const credentials = buildCredentialPayload()
    if (credentials) payload.credentials = credentials
    editingId.value ? await myResourcesApi.accounts.update(editingId.value, payload) : await myResourcesApi.accounts.create(payload)
    editorOpen.value = false
    appStore.showSuccess(t('common.saved'))
    await reload()
  } catch (error) {
    editorError.value = extractApiErrorMessage(error, mr('messages.saveAccountFailed'))
  } finally {
    saving.value = false
  }
}
function parseOAuthCallback(value: string) { const trimmed = value.trim(); if (!trimmed.includes('://') && !trimmed.includes('?')) return { code: trimmed, state: '' }; const code = trimmed.match(/[?&]code=([^&]+)/)?.[1] || ''; const state = trimmed.match(/[?&]state=([^&]+)/)?.[1] || ''; return { code: decodeURIComponent(code), state: decodeURIComponent(state) } }
function applyOAuthResult(result: ResourceItem) { if (!result.credentials || typeof result.credentials !== 'object') throw new Error(mr('messages.oauthCredentialsMissing')); form.credentials_text = JSON.stringify(result.credentials, null, 2); credentialsTouched.value = true; const currentExtra = JSON.parse(form.extra_text || '{}'); form.extra_text = JSON.stringify({ ...currentExtra, ...(result.extra || {}) }, null, 2); if (!form.name.trim() && result.suggested_name) form.name = String(result.suggested_name); oauth.error = ''; appStore.showSuccess(mr('messages.oauthCredentialsApplied')) }
async function generateOAuthURL() { oauth.loading = true; oauth.error = ''; try { const result = await myResourcesApi.accounts.oauth.authURL({ platform: form.platform, proxy_id: form.proxy_id || undefined, setup_token: form.type === 'setup-token', project_id: form.platform === 'gemini' ? oauth.project_id || undefined : undefined, oauth_type: form.platform === 'gemini' ? oauth.oauth_type : undefined, tier_id: form.platform === 'gemini' ? oauth.tier_id || undefined : undefined }); oauth.auth_url = String(result.auth_url || ''); oauth.session_id = String(result.session_id || ''); oauth.state = String(result.state || ''); oauth.callback = ''; if (!oauth.auth_url || !oauth.session_id) throw new Error(mr('messages.oauthUrlMissing')) } catch (error) { oauth.error = extractApiErrorMessage(error, mr('messages.generateAuthUrlFailed')) } finally { oauth.loading = false } }
async function exchangeOAuthCode() { const callback = parseOAuthCallback(oauth.callback); if (!callback.code || !oauth.session_id) return; oauth.loading = true; oauth.error = ''; try { const result = await myResourcesApi.accounts.oauth.exchange({ platform: form.platform, proxy_id: form.proxy_id || undefined, setup_token: form.type === 'setup-token', session_id: oauth.session_id, code: callback.code, state: callback.state || oauth.state || undefined, oauth_type: form.platform === 'gemini' ? oauth.oauth_type : undefined, tier_id: form.platform === 'gemini' ? oauth.tier_id || undefined : undefined }); applyOAuthResult(result); oauth.session_id = '' } catch (error) { oauth.error = extractApiErrorMessage(error, mr('messages.completeAuthorizationFailed')) } finally { oauth.loading = false } }
async function exchangeOAuthCookie() { if (!oauth.session_key.trim()) return; oauth.loading = true; oauth.error = ''; try { const result = await myResourcesApi.accounts.oauth.cookie({ proxy_id: form.proxy_id || undefined, setup_token: form.type === 'setup-token', session_key: oauth.session_key.trim() }); applyOAuthResult(result); oauth.session_key = '' } catch (error) { oauth.error = extractApiErrorMessage(error, mr('messages.sessionKeyAuthorizationFailed')) } finally { oauth.loading = false } }
async function copyOAuthURL() { try { await navigator.clipboard.writeText(oauth.auth_url); appStore.showSuccess(mr('messages.authUrlCopied')) } catch (error) { oauth.error = extractApiErrorMessage(error, mr('messages.copyFailed')) } }
async function testAccount(row: ResourceItem) { try { await myResourcesApi.accounts.test(Number(row.id)); appStore.showSuccess(mr('messages.testSuccess')); await reload() } catch (error) { appStore.showError(extractApiErrorMessage(error, mr('messages.testFailed'))) } }
async function refreshAccount(row: ResourceItem) { try { await myResourcesApi.accounts.refresh(Number(row.id)); appStore.showSuccess(mr('messages.accountRefreshed')); await reload() } catch (error) { appStore.showError(extractApiErrorMessage(error, mr('messages.refreshFailed'))) } }
async function toggleSchedulable(row: ResourceItem) { try { await myResourcesApi.accounts.setSchedulable(Number(row.id), !row.schedulable); row.schedulable = !row.schedulable } catch (error) { appStore.showError(extractApiErrorMessage(error, mr('messages.scheduleUpdateFailed'))) } }
async function deleteAccount(row: ResourceItem) { if (!window.confirm(mr('messages.deleteAccountConfirm', { name: row.name }))) return; try { await myResourcesApi.accounts.delete(Number(row.id)); await reload() } catch (error) { appStore.showError(extractApiErrorMessage(error, mr('messages.deleteFailed'))) } }
async function batchRefresh() { await runSelected(row => myResourcesApi.accounts.refresh(row)) }
async function batchClearErrors() { await runSelected(row => myResourcesApi.accounts.clearError(row)) }
async function runSelected(action: (id: number) => Promise<unknown>) { const results = await Promise.allSettled(selectedIds.value.map(action)); const failed = results.filter(result => result.status === 'rejected').length; failed ? appStore.showError(mr('messages.batchPartialFailed', { count: failed })) : appStore.showSuccess(mr('messages.batchCompleted')); await reload() }
async function submitBatchEdit() { try { await myResourcesApi.accounts.batchUpdate(selectedIds.value, { status: batchForm.status, priority: Number(batchForm.priority), notes: batchForm.notes }); batchOpen.value = false; selectedIds.value = []; await reload() } catch (error) { appStore.showError(extractApiErrorMessage(error, mr('messages.batchFailed'))) } }
function openImport() { textDialogMode.value = 'import'; textDialogValue.value = JSON.stringify({ accounts: [], proxies: [] }, null, 2); textDialogOpen.value = true; showTools.value = false }
function openCodexSession() { textDialogMode.value = 'codex-session'; textDialogValue.value = ''; textDialogOpen.value = true; showTools.value = false }
function openCodexPAT() { textDialogMode.value = 'codex-pat'; textDialogValue.value = ''; textDialogOpen.value = true; showTools.value = false }
async function submitTextDialog() { saving.value = true; editorError.value = ''; try { if (textDialogMode.value === 'import') await myResourcesApi.accounts.import(JSON.parse(textDialogValue.value)); else if (textDialogMode.value === 'codex-session') await myResourcesApi.accounts.importCodexSessions({ content: textDialogValue.value }); else await myResourcesApi.accounts.importCodexPAT({ access_token: textDialogValue.value }); textDialogOpen.value = false; await reload() } catch (error) { editorError.value = extractApiErrorMessage(error, mr('messages.importFailed')) } finally { saving.value = false } }
async function exportAccounts() { const data = await myResourcesApi.accounts.export({ ids: selectedIds.value, include_proxies: true }); const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' }); const link = document.createElement('a'); link.href = URL.createObjectURL(blob); link.download = 'my-accounts.json'; link.click(); URL.revokeObjectURL(link.href); showTools.value = false }
function handleSort(key: string, order: 'asc' | 'desc') { params.sort_by = key === 'platform_type' ? 'platform' : key; params.sort_order = order; reload() }
function toggleSelected(id: number) { selectedIds.value = selectedIds.value.includes(id) ? selectedIds.value.filter(value => value !== id) : [...selectedIds.value, id] }
function toggleAll() { selectedIds.value = allSelected.value ? [] : accounts.value.map(row => Number(row.id)) }
function isColumnVisible(key: string) { return !hiddenColumns.value.has(key) }
function toggleColumn(key: string) { const next = new Set(hiddenColumns.value); next.has(key) ? next.delete(key) : next.add(key); hiddenColumns.value = next; localStorage.setItem('my-account-hidden-columns', JSON.stringify([...next])) }
function changePage(page: number) { pagination.page = page; reload() }
function changePageSize(size: number) { pagination.page_size = size; pagination.page = 1; reload() }
function setAutoRefreshInterval(seconds: number) { autoRefreshInterval.value = seconds; countdown.value = seconds; showAutoRefresh.value = false }
function statusLabel(status: unknown) { const key = String(status || 'inactive'); return t(`admin.accounts.status.${key}`) }
function accountTypeLabel(type: unknown) {
  const value = String(type || '')
  if (value === 'oauth') return 'OAuth'
  if (value === 'setup-token') return 'Setup Token'
  if (value === 'apikey') return 'API Key'
  if (value === 'service_account') return mr('fields.serviceAccount')
  return value
}
function credentialEmail(row: ResourceItem) { const extra = row.extra as ResourceItem | undefined; const credentials = row.credentials as ResourceItem | undefined; return String(extra?.email || credentials?.email || '') }
function formatDate(value: unknown) { if (!value) return '-'; const date = new Date(String(value)); return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString() }
function toDateTimeLocal(value: unknown) { if (!value) return ''; const date = new Date(String(value)); const offset = date.getTimezoneOffset() * 60000; return new Date(date.getTime() - offset).toISOString().slice(0, 16) }

watch(autoRefreshEnabled, enabled => { countdown.value = autoRefreshInterval.value; if (!enabled) return })
onMounted(async () => { const saved = localStorage.getItem('my-account-hidden-columns'); if (saved) { try { hiddenColumns.value = new Set(JSON.parse(saved)) } catch { localStorage.removeItem('my-account-hidden-columns') } } hiddenColumns.value.delete('usage'); await loadReferences(); await reload(); timer = setInterval(() => { if (!autoRefreshEnabled.value) return; countdown.value -= 1; if (countdown.value <= 0) { countdown.value = autoRefreshInterval.value; reload() } }, 1000) })
onBeforeUnmount(() => { if (timer) clearInterval(timer) })
</script>

<style scoped>
.account-menu-item { @apply flex w-full items-center justify-between gap-3 rounded-md px-3 py-2 text-left text-sm text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-dark-700; }
.icon-action { @apply flex h-8 w-8 items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-800 dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-white; }
.field { @apply block; }
.field > span { @apply mb-1 block text-sm text-gray-700 dark:text-dark-100; }
</style>
