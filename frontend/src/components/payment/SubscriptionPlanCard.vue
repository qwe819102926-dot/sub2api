<template>
  <div
    :class="[
      'group relative flex min-h-[390px] flex-col overflow-hidden rounded-xl border bg-white shadow-sm transition-shadow hover:shadow-md dark:bg-dark-800',
      borderClass,
    ]"
  >
    <div class="flex flex-1 flex-col p-4 md:p-5">
      <div :class="['mb-4 flex min-h-[58px] flex-wrap items-center gap-2 rounded-lg px-3 py-2.5', badgeLightClass]">
        <div :class="['flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-white/75 dark:bg-black/10', iconClass]">
          <span class="text-xl font-bold" aria-hidden="true">{{ platformInitial }}</span>
        </div>
        <div class="min-w-0 flex-1 basis-[min(100%,12rem)]">
          <h3
            :title="plan.name"
            class="h-12 min-w-0 break-words [overflow-wrap:anywhere] text-base font-bold leading-5 text-gray-900 dark:text-white line-clamp-2"
          >
            {{ plan.name }}
          </h3>
          <p v-if="plan.description" class="mt-1 text-xs leading-4 text-gray-500 dark:text-dark-400 line-clamp-1">
            {{ plan.description }}
          </p>
        </div>
        <span :class="['max-w-full shrink-0 rounded-full px-2 py-0.5 text-[10px] font-medium', badgeLightClass]">{{ pLabel }} <span class="text-[10px] text-gray-400 dark:text-gray-500">/ {{ validitySuffix }}</span></span>
        <span :class="['shrink-0 rounded px-2 py-1 text-[10px] font-semibold', discountClass]" v-if="discountText">{{ discountText }}</span>
      </div>

      <div class="mb-1 flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
        <span :class="['text-3xl font-extrabold leading-none', textClass]">{{ planCurrencySymbol }}{{ plan.price }}<small v-if="plan.currency" class="ml-0.5 text-xs font-medium">{{ plan.currency }}</small></span>
        <span class="text-sm text-gray-500 dark:text-gray-400">/ {{ validitySuffix }}</span>
        <span v-if="plan.original_price" class="text-xs text-gray-400 line-through dark:text-dark-500">{{ planCurrencySymbol }}{{ plan.original_price }}<small v-if="plan.currency" class="ml-0.5 no-underline">{{ plan.currency }}</small></span>
      </div>
      <div class="mb-4 min-h-5 text-xs text-gray-500 dark:text-gray-400">
        {{ pLabel }}
      </div>

      <div :class="['mb-3 flex min-h-8 items-center gap-2 rounded-md px-3 py-1.5 text-xs', badgeLightClass]">
        <span :class="['font-semibold', iconClass]" aria-hidden="true">◈</span>
        <span v-if="plan.daily_limit_usd != null">{{ t('payment.planCard.dailyLimit') }}: ${{ plan.daily_limit_usd }}</span>
        <span v-else-if="plan.weekly_limit_usd != null">{{ t('payment.planCard.weeklyLimit') }}: ${{ plan.weekly_limit_usd }}</span>
        <span v-else-if="plan.monthly_limit_usd != null">{{ t('payment.planCard.monthlyLimit') }}: ${{ plan.monthly_limit_usd }}</span>
        <span v-else>{{ t('payment.planCard.quota') }}: {{ t('payment.planCard.unlimited') }}</span>
        <span class="text-gray-500 dark:text-gray-400">{{ t('payment.planCard.rate') }} {{ rateDisplay }}</span>
      </div>

      <div class="mb-3 space-y-2 text-sm text-gray-700 dark:text-gray-300">
        <div v-if="plan.daily_limit_usd != null" class="flex items-center gap-2">
          <span :class="['text-base font-bold leading-none', iconClass]">✓</span>
          <span>{{ t('payment.planCard.dailyLimit') }}: ${{ plan.daily_limit_usd }}</span>
        </div>
        <div v-if="plan.weekly_limit_usd != null" class="flex items-center gap-2">
          <span :class="['text-base font-bold leading-none', iconClass]">✓</span>
          <span>{{ t('payment.planCard.weeklyLimit') }}: ${{ plan.weekly_limit_usd }}</span>
        </div>
        <div v-if="plan.monthly_limit_usd != null" class="flex items-center gap-2">
          <span :class="['text-base font-bold leading-none', iconClass]">✓</span>
          <span>{{ t('payment.planCard.monthlyLimit') }}: ${{ plan.monthly_limit_usd }}</span>
        </div>
        <div v-if="hasPeakRate" class="flex items-start gap-2">
          <span :class="['text-base font-bold leading-none', iconClass]">✓</span>
          <span>{{ t('payment.planCard.peakRate') }}: {{ peakRateDisplay }}</span>
        </div>
        <div v-if="modelScopeLabels.length > 0" class="flex items-center gap-2">
          <span :class="['text-base font-bold leading-none', iconClass]">✓</span>
          <span>{{ t('payment.planCard.models') }}: {{ modelScopeLabels.join(', ') }}</span>
        </div>
      </div>

      <div v-if="plan.features.length > 0" class="mb-4 space-y-2 text-sm text-gray-700 dark:text-gray-300">
        <div v-for="feature in plan.features" :key="feature" class="flex items-start gap-2">
          <span :class="['text-base font-bold leading-none', iconClass]" aria-hidden="true">✓</span>
          <span class="min-w-0 break-words">{{ feature }}</span>
        </div>
      </div>

      <div class="flex-1" />

      <button
        type="button"
        :class="['w-full rounded-lg border py-2.5 text-sm font-semibold transition-colors active:scale-[0.99]', btnClass]"
        @click="emit('select', plan)"
      >
        {{ isRenewal ? t('payment.renewNow') : t('payment.subscribeNow') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SubscriptionPlan } from '@/types/payment'
import type { UserSubscription } from '@/types'
import { useAppStore } from '@/stores/app'
import { hasPeakRate as groupHasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import { planValiditySuffix } from './validity'
import { currencySymbol } from '@/components/payment/currency'
import {
  platformAccentBarClass,
  platformBadgeLightClass,
  platformBorderClass,
  platformTextClass,
  platformIconClass,
  platformButtonClass,
  platformDiscountClass,
  platformLabel,
} from '@/utils/platformColors'

const props = defineProps<{ plan: SubscriptionPlan; activeSubscriptions?: UserSubscription[] }>()
const emit = defineEmits<{ select: [plan: SubscriptionPlan] }>()
const { t } = useI18n()

const platform = computed(() => props.plan.group_platform || '')
const isRenewal = computed(() =>
  props.activeSubscriptions?.some(s => s.group_id === props.plan.group_id && s.status === 'active') ?? false
)

// Derived color classes from central config
const accentClass = computed(() => platformAccentBarClass(platform.value))
const borderClass = computed(() => platformBorderClass(platform.value))
const badgeLightClass = computed(() => platformBadgeLightClass(platform.value))
const textClass = computed(() => platformTextClass(platform.value))
const iconClass = computed(() => platformIconClass(platform.value))
const btnClass = computed(() => platformButtonClass(platform.value))
const discountClass = computed(() => platformDiscountClass(platform.value))
const pLabel = computed(() => platformLabel(platform.value))
const platformInitial = computed(() => pLabel.value.slice(0, 1).toUpperCase())

const discountText = computed(() => {
  if (!props.plan.original_price || props.plan.original_price <= 0) return ''
  const pct = Math.round((1 - props.plan.price / props.plan.original_price) * 100)
  return pct > 0 ? `-${pct}%` : ''
})

const rateDisplay = computed(() => {
  const rate = props.plan.rate_multiplier ?? 1
  return `×${Number(rate.toPrecision(10))}`
})

const appStore = useAppStore()
const planCurrencySymbol = computed(() => currencySymbol(props.plan.currency || 'USD'))

const hasPeakRate = computed(() => groupHasPeakRate(props.plan))

const peakRateDisplay = computed(() => {
  return formatPeakRateWindow(props.plan, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset))
})

const MODEL_SCOPE_LABELS: Record<string, string> = {
  claude: 'Claude',
  gemini_text: 'Gemini',
  gemini_image: 'Imagen',
}

const modelScopeLabels = computed(() => {
  if (platform.value !== 'antigravity') return []
  const scopes = props.plan.supported_model_scopes
  if (!scopes || scopes.length === 0) return []
  return scopes.map(s => MODEL_SCOPE_LABELS[s] || s)
})

const validitySuffix = computed(() => planValiditySuffix(props.plan, t))
</script>
