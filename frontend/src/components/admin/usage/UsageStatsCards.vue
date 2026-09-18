<template>
  <div class="space-y-4">
    <div class="usage-stats-cards grid grid-cols-2 gap-4 lg:grid-cols-4">
      <div class="card p-4 flex items-center gap-3">
        <div class="rounded-lg bg-blue-100 p-2 dark:bg-blue-900/30 text-blue-600">
          <Icon name="document" size="md" />
        </div>
        <div>
          <p class="text-xs font-medium text-gray-500">{{ t('usage.totalRequests') }}</p>
          <p class="text-xl font-bold">{{ stats?.total_requests?.toLocaleString() || '0' }}</p>
          <p class="text-xs text-gray-400">{{ t('usage.inSelectedRange') }}</p>
        </div>
      </div>
      <div class="card p-4 flex items-center gap-3">
        <div class="rounded-lg bg-amber-100 p-2 dark:bg-amber-900/30 text-amber-600"><svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="m21 7.5-9-5.25L3 7.5m18 0-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9" /></svg></div>
        <div>
          <p class="text-xs font-medium text-gray-500">{{ t('usage.totalTokens') }}</p>
          <p class="text-xl font-bold">{{ formatTokens(stats?.total_tokens || 0) }}</p>
          <p class="flex flex-wrap items-center gap-x-1 text-xs text-gray-500">
            <span>{{ t('usage.in') }}: {{ formatTokens(stats?.total_input_tokens || 0) }}</span>
            <span>/</span>
            <span>{{ t('usage.out') }}: {{ formatTokens(stats?.total_output_tokens || 0) }}</span>
            <span>/</span>
            <span class="cache-token-tooltip-trigger group relative inline-flex cursor-help items-center gap-0.5" tabindex="0">
              <span>{{ cacheLabel() }}: {{ formatTokens(stats?.total_cache_tokens || 0) }}</span>
              <svg
                class="h-3.5 w-3.5 text-gray-400"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
              <span
                class="pointer-events-none absolute left-1/2 top-full z-30 mt-2 hidden w-56 -translate-x-1/2 rounded-lg border border-gray-200 bg-white p-3 text-left text-xs text-gray-700 shadow-lg group-hover:block group-focus:block dark:border-dark-600 dark:bg-dark-800 dark:text-dark-200"
              >
                <span class="mb-2 block font-medium text-gray-900 dark:text-white">
                  {{ cacheDetailLabel() }}
                </span>
                <span class="flex items-center justify-between gap-3">
                  <span>{{ t('usage.cacheCreationTokensLabel') }}</span>
                  <span class="tabular-nums">
                    {{ formatTokens(stats?.total_cache_creation_tokens || 0) }}
                  </span>
                </span>
                <span class="mt-1 flex items-center justify-between gap-3">
                  <span>{{ t('usage.cacheReadTokensLabel') }}</span>
                  <span class="tabular-nums">
                    {{ formatTokens(stats?.total_cache_read_tokens || 0) }}
                  </span>
                </span>
              </span>
            </span>
          </p>
        </div>
      </div>
      <div class="card p-4 flex items-center gap-3">
        <div class="rounded-lg bg-green-100 p-2 dark:bg-green-900/30 text-green-600">
          <Icon name="dollar" size="md" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-xs font-medium text-gray-500">{{ t('usage.totalCost') }}</p>
          <p class="text-xl font-bold text-green-600">
            ${{ formatMoney(stats?.total_actual_cost) }}
          </p>
          <p class="text-xs text-gray-400">
            <template v-if="showAccountCost && totalAccountCost != null">
              <span class="text-orange-500">{{ t('usage.accountCost') }} ${{ formatMoney(totalAccountCost) }}</span>
              <span> · </span>
            </template>
            <span>
              {{ t('usage.standardCost') }}
              <span :class="{ 'line-through': strikeStandardCost }">${{ formatMoney(stats?.total_cost) }}</span>
            </span>
          </p>
        </div>
      </div>
      <div class="card p-4 flex items-center gap-3">
        <div class="rounded-lg bg-purple-100 p-2 dark:bg-purple-900/30 text-purple-600">
          <Icon name="clock" size="md" />
        </div>
        <div><p class="text-xs font-medium text-gray-500">{{ t('usage.avgDuration') }}</p><p class="text-xl font-bold">{{ formatDuration(stats?.average_duration_ms || 0) }}</p></div>
      </div>
    </div>

    <div v-if="showProfitCard" class="card p-4">
      <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div class="flex items-center gap-3">
          <div class="rounded-lg bg-indigo-100 p-2 dark:bg-indigo-900/30 text-indigo-600">
            <Icon name="chart" size="md" />
          </div>
          <div>
            <p class="group relative inline-flex cursor-help items-center gap-1 text-xs font-medium text-gray-500" tabindex="0">
              <span>{{ t('usage.rangeProfit') }}</span>
              <svg class="h-3.5 w-3.5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <span class="pointer-events-none absolute left-0 top-full z-30 mt-2 hidden w-72 rounded-lg border border-gray-200 bg-white p-3 text-left text-xs font-normal text-gray-700 shadow-lg group-hover:block group-focus:block dark:border-dark-600 dark:bg-dark-800 dark:text-dark-200">
                {{ t('usage.profitHint') }}
              </span>
            </p>
            <p class="text-xl font-bold tabular-nums" :class="profitColorClass">{{ formatSignedMoney(totalProfit) }}</p>
            <p class="text-xs text-gray-400">{{ t('usage.profitFormula') }}</p>
          </div>
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-3 lg:min-w-[28rem]">
          <div>
            <p class="text-xs font-medium text-gray-500">{{ t('usage.walletSpend') }}</p>
            <p class="text-lg font-semibold tabular-nums">${{ formatMoney(totalWalletCost) }}</p>
          </div>
          <div>
            <p class="text-xs font-medium text-gray-500">{{ t('usage.bonusSpend') }}</p>
            <p class="text-lg font-semibold tabular-nums">${{ formatMoney(totalBonusCost) }}</p>
          </div>
          <div>
            <p class="text-xs font-medium text-gray-500">{{ t('usage.accountCost') }}</p>
            <p class="text-lg font-semibold tabular-nums text-orange-500">${{ formatMoney(totalAccountCost) }}</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminUsageStatsResponse } from '@/api/admin/usage'
import type { UsageStatsResponse } from '@/types'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  stats: (AdminUsageStatsResponse | UsageStatsResponse) | null
  showAccountCost?: boolean
  strikeStandardCost?: boolean
}>(), {
  showAccountCost: true,
  strikeStandardCost: false,
})

const { t } = useI18n()

const adminStats = computed(() => props.stats as AdminUsageStatsResponse | null)
const totalAccountCost = computed(() => adminStats.value?.total_account_cost ?? null)
const totalWalletCost = computed(() => adminStats.value?.total_wallet_cost ?? 0)
const totalBonusCost = computed(() => adminStats.value?.total_bonus_cost ?? 0)
const totalProfit = computed(() => adminStats.value?.total_profit ?? 0)
const showAccountCost = computed(() => props.showAccountCost)
const showProfitCard = computed(() => props.showAccountCost)
const strikeStandardCost = computed(() => props.strikeStandardCost)
const profitColorClass = computed(() => {
  if (totalProfit.value > 0) return 'text-green-600'
  if (totalProfit.value < 0) return 'text-red-600'
  return 'text-gray-700 dark:text-gray-200'
})

const formatDuration = (ms: number) =>
  ms < 1000 ? `${ms.toFixed(0)}ms` : `${(ms / 1000).toFixed(2)}s`

const formatTokens = (value: number) => {
  if (value >= 1e9) return (value / 1e9).toFixed(2) + 'B'
  if (value >= 1e6) return (value / 1e6).toFixed(2) + 'M'
  if (value >= 1e3) return (value / 1e3).toFixed(2) + 'K'
  return value.toLocaleString()
}

const formatMoney = (value: number | null | undefined) => (value || 0).toFixed(4)

const formatSignedMoney = (value: number) => {
  const formatted = Math.abs(value).toFixed(4)
  return value < 0 ? `-$${formatted}` : `$${formatted}`
}

const cacheLabel = () => t('usage.cacheTotal')
const cacheDetailLabel = () => t('usage.cacheBreakdown')
</script>
