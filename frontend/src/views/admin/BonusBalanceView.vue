<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('bonusBalanceAdmin.description') }}</p>
        <button class="btn btn-secondary" :disabled="loading" @click="load(page)">{{ t('common.refresh') }}</button>
      </div>
      <p v-if="error" role="alert" class="rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }}</p>
      <section class="card space-y-3 p-6">
        <h2 class="font-semibold">{{ t('bonusBalanceAdmin.conversionTitle') }}</h2>
        <p class="text-sm leading-6 text-gray-500 dark:text-dark-400">{{ t('bonusBalanceAdmin.conversionHint') }}</p>
        <p v-if="conversion?.completed" class="text-sm text-emerald-600" data-testid="conversion-completed">{{ t('bonusBalanceAdmin.conversionReceipt', { users: conversion.users, before: conversion.before_total, after: conversion.after_total, date: conversion.completed_at ? formatDateTime(conversion.completed_at) : '—' }) }}</p>
        <button class="btn btn-secondary" data-testid="conversion-open" :disabled="converting || conversionLoading || conversion?.completed" @click="openConversion">{{ t(conversion?.completed ? 'bonusBalanceAdmin.conversionDone' : 'bonusBalanceAdmin.conversionButton') }}</button>
      </section>
      <div v-if="!report && loading" role="status" class="card p-12 text-center text-gray-500">{{ t('common.loading') }}</div>
      <template v-if="report">
        <section class="grid gap-4 md:grid-cols-3">
          <div v-for="metric in metrics" :key="metric.key" class="card p-6">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t(`bonusBalanceAdmin.${metric.key}`) }}</p>
            <p class="mt-3 text-3xl font-semibold tabular-nums" :class="metric.color" :data-testid="metric.key">{{ formatCurrency(metric.value) }}</p>
            <p class="mt-2 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ t(`bonusBalanceAdmin.${metric.key}Hint`) }}</p>
          </div>
        </section>

        <div class="rounded-xl border border-gray-200 bg-gray-50 p-4 text-sm leading-6 text-gray-600 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-300">
          <p>{{ t('bonusBalanceAdmin.reconcile', { adjustments: formatCurrency(report.summary.adjustments), historical: formatCurrency(report.summary.historical_outflow) }) }}</p>
          <p>{{ t('bonusBalanceAdmin.equation') }}</p>
          <p>{{ t('bonusBalanceAdmin.historyHint') }}</p>
          <p v-if="report.summary.deleted_user_remaining > 0">{{ t('bonusBalanceAdmin.deletedHint', { amount: formatCurrency(report.summary.deleted_user_remaining) }) }}</p>
        </div>
        <p v-if="Math.abs(report.summary.reconciliation_difference) > 0.00000001" role="alert" class="rounded-xl bg-amber-50 p-4 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-300">{{ t('bonusBalanceAdmin.variance', { amount: formatCurrency(report.summary.reconciliation_difference) }) }}</p>

        <section class="card overflow-hidden">
          <div class="border-b border-gray-100 p-6 dark:border-dark-700">
            <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('bonusBalanceAdmin.sourcesTitle') }}</h2>
            <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-dark-400">{{ t('bonusBalanceAdmin.sourceHint') }}</p>
          </div>
          <div v-if="report.sources.length" class="overflow-x-auto">
            <table class="w-full min-w-[540px] text-left text-sm">
              <thead><tr class="border-b border-gray-100 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                <th class="px-6 py-3 font-medium">{{ t('bonusBalanceAdmin.source') }}</th>
                <th class="px-6 py-3 font-medium">{{ t('bonusBalanceAdmin.issued') }}</th>
                <th class="px-6 py-3 font-medium">{{ t('bonusBalanceAdmin.share') }}</th>
                <th class="px-6 py-3 font-medium">{{ t('bonusBalanceAdmin.grants') }}</th>
                <th class="px-6 py-3 font-medium">{{ t('bonusBalanceAdmin.users') }}</th>
              </tr></thead>
              <tbody><tr v-for="source in report.sources" :key="source.source" class="border-b border-gray-100 last:border-0 dark:border-dark-800">
                <td class="px-6 py-4"><button class="text-primary-600 hover:underline dark:text-primary-400" @click="selectSource(source.source)">{{ sourceLabel(source.source) }}</button></td>
                <td class="px-6 py-4 font-medium tabular-nums">{{ formatCurrency(source.amount) }}</td>
                <td class="px-6 py-4"><div class="flex items-center gap-3"><div class="h-1.5 w-20 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700"><div class="h-full rounded-full bg-emerald-500" :style="{ width: `${share(source.amount)}%` }" /></div><span class="tabular-nums">{{ share(source.amount).toFixed(1) }}%</span></div></td>
                <td class="px-6 py-4 tabular-nums">{{ source.grants.toLocaleString() }}</td>
                <td class="px-6 py-4 tabular-nums">{{ source.users.toLocaleString() }}</td>
              </tr></tbody>
            </table>
          </div>
          <p v-else class="p-8 text-center text-sm text-gray-500">{{ t('bonusBalanceAdmin.empty') }}</p>
        </section>

        <section class="card overflow-hidden">
          <div class="space-y-4 border-b border-gray-100 p-6 dark:border-dark-700">
            <div><h2 class="font-semibold text-gray-900 dark:text-white">{{ t('bonusBalanceAdmin.recordsTitle') }}</h2><p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('bonusBalanceAdmin.filterHint') }}</p></div>
            <form class="flex flex-wrap gap-3" @submit.prevent="load(1)">
              <input v-model="filters.search" class="input min-w-0 flex-1 sm:min-w-52" :aria-label="t('bonusBalanceAdmin.search')" :placeholder="t('bonusBalanceAdmin.search')" />
              <select v-model="filters.source" class="input w-full sm:w-44" :aria-label="t('bonusBalanceAdmin.source')" @change="load(1)">
                <option value="">{{ t('bonusBalanceAdmin.allSources') }}</option>
                <option v-for="source in sourceOptions" :key="source" :value="source">{{ sourceLabel(source) }}</option>
              </select>
              <select v-model="filters.kind" class="input w-full sm:w-44" :aria-label="t('bonusBalanceAdmin.kind')" @change="load(1)">
                <option value="">{{ t('bonusBalanceAdmin.allKinds') }}</option>
                <option value="grant">{{ t('bonusBalanceAdmin.grant') }}</option>
                <option value="consumption">{{ t('bonusBalanceAdmin.consumption') }}</option>
                <option value="adjustment">{{ t('bonusBalanceAdmin.adjustment') }}</option>
              </select>
              <button class="btn btn-primary" :disabled="loading" type="submit">{{ t('bonusBalanceAdmin.query') }}</button>
            </form>
          </div>
          <div class="overflow-x-auto" :aria-busy="loading">
            <table class="w-full min-w-[800px] text-left text-sm">
              <thead><tr class="border-b border-gray-100 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                <th v-for="label in ['account', 'source', 'movement', 'after', 'recordType', 'date']" :key="label" class="px-6 py-3 font-medium">{{ t(`bonusBalanceAdmin.${label}`) }}</th>
              </tr></thead>
              <tbody><tr v-for="record in report.records" :key="record.id" class="border-b border-gray-100 last:border-0 dark:border-dark-800">
                <td class="px-6 py-4"><p class="max-w-60 truncate" :title="record.email">{{ record.email || '—' }}</p><p class="mt-1 text-xs text-gray-400">ID {{ record.user_id }}</p></td>
                <td class="px-6 py-4">{{ sourceLabel(record.source) }}</td>
                <td class="px-6 py-4 font-medium tabular-nums" :class="record.amount > 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-orange-600 dark:text-orange-400'">{{ record.amount > 0 ? '+' : '' }}{{ formatCurrency(record.amount) }}</td>
                <td class="px-6 py-4 tabular-nums">{{ record.balance_after == null ? '—' : formatCurrency(record.balance_after) }}</td>
                <td class="px-6 py-4 text-xs text-gray-500 dark:text-dark-400">{{ t(record.historical ? 'bonusBalanceAdmin.historical' : record.source === 'consumption' ? 'bonusBalanceAdmin.dailyAggregate' : 'bonusBalanceAdmin.live') }}</td>
                <td class="px-6 py-4 whitespace-nowrap text-gray-500 dark:text-dark-400">{{ formatDateTime(record.updated_at) }}</td>
              </tr></tbody>
            </table>
            <p v-if="report.records.length === 0" class="p-10 text-center text-sm text-gray-500">{{ t('bonusBalanceAdmin.empty') }}</p>
          </div>
          <div class="flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 px-6 py-4 text-sm dark:border-dark-700">
            <span class="text-gray-500">{{ t('bonusBalanceAdmin.total', { total: report.total }) }}</span>
            <div class="flex items-center gap-3">
              <button class="btn btn-secondary btn-sm" :disabled="loading || page <= 1" @click="load(page - 1)">{{ t('pagination.previous') }}</button>
              <span>{{ page }} / {{ Math.max(1, Math.ceil(report.total / report.page_size)) }}</span>
              <button class="btn btn-secondary btn-sm" :disabled="loading || page * report.page_size >= report.total" @click="load(page + 1)">{{ t('pagination.next') }}</button>
            </div>
          </div>
        </section>
      </template>
      <button v-else-if="!loading" class="btn btn-primary" @click="load(1)">{{ t('bonusBalanceAdmin.retry') }}</button>
    </div>
    <BaseDialog :show="conversionDialog" :title="t('bonusBalanceAdmin.conversionTitle')" width="normal" :close-on-escape="!converting" :show-close-button="!converting" @close="closeConversion">
      <div class="space-y-4">
        <p class="text-sm leading-6">{{ t('bonusBalanceAdmin.conversionWarning') }}</p>
        <p v-if="conversion" class="rounded-lg bg-amber-50 p-4 text-sm text-amber-900 dark:bg-amber-900/20 dark:text-amber-200" data-testid="conversion-preview">{{ t('bonusBalanceAdmin.conversionPreview', { users: conversion.users, before: conversion.before_total, after: conversion.after_total }) }}</p>
        <label class="block text-sm" for="conversion-confirmation">{{ t('bonusBalanceAdmin.conversionConfirmLabel') }}</label>
        <input id="conversion-confirmation" v-model="confirmation" class="input w-full" autocomplete="off" :disabled="converting" placeholder="DIVIDE_BY_10" />
        <p v-if="conversionError" role="alert" class="text-sm text-red-600">{{ conversionError }}</p>
      </div>
      <template #footer>
        <button class="btn btn-secondary" :disabled="converting" @click="closeConversion">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" data-testid="conversion-submit" :disabled="converting || confirmation !== 'DIVIDE_BY_10' || !conversion || conversion.completed || conversion.users === 0" @click="executeConversion">{{ t(converting ? 'common.processing' : 'bonusBalanceAdmin.conversionExecute') }}</button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getBonusBalanceReport, getBonusBalanceConversion, convertBonusBalances, type BonusBalanceReport, type BonusBalanceConversion } from '@/api/admin/bonusBalance'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const report = ref<BonusBalanceReport | null>(null)
const page = ref(1)
const loading = ref(false)
const error = ref('')
const filters = reactive({ search: '', source: '', kind: '' })
const sourceOptions = ['signup', 'invite', 'checkin', 'recharge', 'consumption_reward', 'lottery', 'admin', 'consumption', 'historical_credit', 'historical_outflow', 'account_removed', 'conversion', 'other']
const conversion = ref<BonusBalanceConversion | null>(null)
const conversionDialog = ref(false)
const conversionLoading = ref(true)
const converting = ref(false)
const confirmation = ref('')
const conversionError = ref('')

async function openConversion() {
  if (conversionLoading.value || converting.value || conversion.value?.completed) return
  conversionLoading.value = true
  error.value = ''
  try {
    conversion.value = await getBonusBalanceConversion()
    if (!conversion.value.completed) {
      confirmation.value = ''
      conversionError.value = ''
      conversionDialog.value = true
    }
  } catch (cause) {
    error.value = extractApiErrorMessage(cause, t('bonusBalanceAdmin.conversionFailed'))
  } finally {
    conversionLoading.value = false
  }
}
function closeConversion() {
  if (!converting.value) conversionDialog.value = false
}
async function executeConversion() {
  if (converting.value || confirmation.value !== 'DIVIDE_BY_10' || !conversion.value || conversion.value.completed || conversion.value.users === 0) return
  converting.value = true
  conversionError.value = ''
  try {
    conversion.value = await convertBonusBalances(confirmation.value)
    conversionDialog.value = false
    await load(1)
  } catch (cause) {
    conversionError.value = extractApiErrorMessage(cause, t('bonusBalanceAdmin.conversionFailed'))
    // A timeout may follow a successful commit. Reconcile before allowing retry.
    try {
      conversion.value = await getBonusBalanceConversion()
      if (conversion.value.completed) {
        conversionDialog.value = false
        await load(1)
      }
    } catch { /* The server's permanent guard still prevents a duplicate conversion. */ }
  } finally {
    converting.value = false
  }
}
let revision = 0
const metrics = computed(() => report.value ? [
  { key: 'issued', value: report.value.summary.issued, color: 'text-gray-900 dark:text-white' },
  { key: 'consumed', value: report.value.summary.consumed, color: 'text-orange-600 dark:text-orange-400' },
  { key: 'remaining', value: report.value.summary.remaining, color: 'text-emerald-600 dark:text-emerald-400' },
] : [])
const sourceLabel = (source: string) => t(`bonusBalanceAdmin.sources.${sourceOptions.includes(source) ? source : 'other'}`)
const share = (amount: number) => report.value && report.value.summary.issued > 0 ? amount / report.value.summary.issued * 100 : 0
async function load(target: number) {
  const request = ++revision
  loading.value = true
  error.value = ''
  try {
    const data = await getBonusBalanceReport({ ...filters, page: target, page_size: 20 })
    if (request !== revision) return
    report.value = data
    page.value = data.page
  } catch (cause) {
    if (request === revision) error.value = extractApiErrorMessage(cause, t('bonusBalanceAdmin.loadFailed'))
  } finally {
    if (request === revision) loading.value = false
  }
}
function selectSource(source: string) { filters.source = source; filters.kind = 'grant'; void load(1) }
onMounted(() => {
  void load(1)
  void getBonusBalanceConversion()
    .then(value => { conversion.value = value })
    .catch(() => { /* Opening the dialog retries the status request. */ })
    .finally(() => { conversionLoading.value = false })
})
onBeforeUnmount(() => { revision++ })
</script>
