<template>
  <section class="space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700">
    <div class="flex items-center justify-between gap-3">
      <h3 class="font-medium">{{ t('registrationIPMonitor.title') }}</h3>
      <button type="button" class="btn btn-secondary" :disabled="loading" @click="load(page)">{{ t('common.refresh') }}</button>
    </div>
    <p class="text-sm text-gray-500">{{ t('registrationIPMonitor.hint') }}</p>
    <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
    <template v-if="report">
      <div class="flex flex-wrap items-end gap-3">
        <label class="flex items-center gap-2 text-sm"><input v-model="config.enabled" type="checkbox" />{{ t('registrationIPMonitor.enabled') }}</label>
        <label class="text-sm">{{ t('registrationIPMonitor.window') }}<input v-model.number="config.window_hours" class="input mt-1 w-28" type="number" min="1" max="720" /></label>
        <label class="text-sm">{{ t('registrationIPMonitor.threshold') }}<input v-model.number="config.max_accounts" class="input mt-1 w-28" type="number" min="1" max="10000" /></label>
        <label class="text-sm">{{ t('registrationIPMonitor.duration') }}<input v-model.number="config.ban_hours" class="input mt-1 w-28" type="number" min="1" max="8760" /></label>
        <button type="button" class="btn btn-primary" :disabled="saving || loading" @click="save">{{ t('registrationIPMonitor.save') }}</button>
      </div>
      <p v-if="saved" role="status" class="text-sm text-emerald-600">{{ t('registrationIPMonitor.saved') }}</p>
      <p class="text-sm text-gray-500">{{ t('registrationIPMonitor.rule', report.config) }}</p>
      <div class="flex gap-2">
        <input v-model="search" @keydown.enter.stop.prevent="load(1)" class="input flex-1" :placeholder="t('registrationIPMonitor.search')" :aria-label="t('registrationIPMonitor.search')" />
        <button type="button" class="btn btn-secondary" :disabled="loading">{{ t('common.search') }}</button>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full min-w-[800px] text-left text-sm">
          <thead><tr class="border-b"><th v-for="key in ['ip', 'total', 'recent', 'last', 'state', 'blockedAt', 'until', 'actions']" :key="key" class="p-3">{{ t(`registrationIPMonitor.${key}`) }}</th></tr></thead>
          <tbody><tr v-for="row in report.rows" :key="row.ip" class="border-b dark:border-dark-700">
            <td class="p-3 font-mono"><button type="button" class="text-primary-600 hover:underline" @click="selectIP(row.ip)">{{ row.ip }}</button></td>
            <td class="p-3">{{ row.total }}</td><td class="p-3">{{ row.recent }}</td><td class="p-3">{{ formatDateTime(row.last_registered_at) }}</td>
            <td class="p-3" :class="row.active ? 'text-red-600' : 'text-emerald-600'">{{ t(`registrationIPMonitor.${row.active ? 'blocked' : 'normal'}`) }}</td>
            <td class="p-3">{{ row.blocked_at ? formatDateTime(row.blocked_at) : '—' }}</td>
            <td class="p-3">{{ row.blocked_until ? formatDateTime(row.blocked_until) : '—' }}</td>
            <td class="p-3"><button type="button" class="text-primary-600 hover:underline" @click="selectIP(row.ip)">{{ t('registrationIPMonitor.accounts') }}</button><button type="button" v-if="row.active" class="ml-3 text-red-600 hover:underline" :disabled="busyIP === row.ip" @click="unblock(row.ip)">{{ t('registrationIPMonitor.unblock') }}</button></td>
          </tr></tbody>
        </table>
        <p v-if="!report.rows.length" class="p-6 text-center text-sm text-gray-500">{{ t('registrationIPMonitor.empty') }}</p>
      </div>
      <div class="flex items-center justify-between text-sm"><span>{{ t('registrationIPMonitor.ipCount', { count: report.total }) }}</span><div class="flex items-center gap-3"><button type="button" :disabled="page <= 1 || loading" @click="load(page - 1)">{{ t('pagination.previous') }}</button><span>{{ page }} / {{ Math.max(1, Math.ceil(report.total / 20)) }}</span><button type="button" :disabled="page * 20 >= report.total || loading" @click="load(page + 1)">{{ t('pagination.next') }}</button></div></div>
      <div v-if="selectedIP" class="space-y-3 rounded-lg bg-gray-50 p-4 dark:bg-dark-800">
        <h4 class="font-medium">{{ t('registrationIPMonitor.accountTitle', { ip: selectedIP, count: report.account_total }) }}</h4>
        <p class="text-xs text-gray-500">{{ t('registrationIPMonitor.accountLimit') }}</p>
        <div class="overflow-x-auto"><table class="w-full min-w-[600px] text-left text-sm"><thead><tr><th class="p-2">ID</th><th class="p-2">{{ t('registrationIPMonitor.email') }}</th><th class="p-2">{{ t('registrationIPMonitor.source') }}</th><th class="p-2">{{ t('registrationIPMonitor.registered') }}</th><th class="p-2">{{ t('registrationIPMonitor.state') }}</th></tr></thead><tbody><tr v-for="account in report.accounts" :key="account.user_id"><td class="p-2">{{ account.user_id }}</td><td class="p-2">{{ account.email }}</td><td class="p-2">{{ account.source || 'email' }}</td><td class="p-2">{{ formatDateTime(account.created_at) }}</td><td class="p-2">{{ account.deleted ? t('registrationIPMonitor.deleted') : account.status }}</td></tr></tbody></table></div>
      </div>
    </template>
    <p v-else-if="loading" role="status" class="text-sm text-gray-500">{{ t('common.loading') }}</p>
  </section>
</template>
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getRegistrationIPReport, saveRegistrationIPConfig, unblockRegistrationIP, type RegistrationIPReport } from '@/api/admin/registrationIPMonitor'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
const { t } = useI18n()
const report = ref<RegistrationIPReport | null>(null)
const config = reactive({ enabled: true, window_hours: 24, max_accounts: 5, ban_hours: 24 })
const loading = ref(false), saving = ref(false), saved = ref(false)
const search = ref(''), selectedIP = ref(''), busyIP = ref(''), error = ref(''), page = ref(1)
let revision = 0
async function load(target = 1, syncConfig = false) {
  const current = ++revision
  loading.value = true; error.value = ''
  try {
    const value = await getRegistrationIPReport({ search: search.value, ip: selectedIP.value, page: target })
    if (current !== revision) return
    if (!report.value || syncConfig) Object.assign(config, value.config)
    report.value = value; page.value = value.page
  } catch (cause) { if (current === revision) error.value = extractApiErrorMessage(cause, t('registrationIPMonitor.failed')) }
  finally { if (current === revision) loading.value = false }
}
async function save() {
  saving.value = true; saved.value = false; error.value = ''
  try { await saveRegistrationIPConfig({ ...config }); await load(page.value, true); saved.value = !error.value }
  catch (cause) { error.value = extractApiErrorMessage(cause, t('registrationIPMonitor.failed')) }
  finally { saving.value = false }
}
function selectIP(ip: string) { selectedIP.value = ip; void load(page.value) }
async function unblock(ip: string) {
  busyIP.value = ip; error.value = ''
  try { await unblockRegistrationIP(ip); await load(page.value) }
  catch (cause) { error.value = extractApiErrorMessage(cause, t('registrationIPMonitor.failed')) }
  finally { busyIP.value = '' }
}
onMounted(() => { void load() })
</script>
