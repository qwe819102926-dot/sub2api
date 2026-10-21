<template>
  <section v-if="visible" id="consumption-reward" class="rounded-lg border border-red-200 bg-white p-5 shadow-sm dark:border-red-900/50 dark:bg-dark-800">
    <div v-if="status.consumption_reward.enabled" class="min-w-0">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-center gap-3">
          <div class="rounded-lg bg-red-50 p-2 text-red-500 dark:bg-red-900/20 dark:text-red-300"><Icon name="gift" size="md" /></div>
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('payment.activities.consumptionTitle') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('payment.activities.currentSpent', { amount: format(status.consumption_reward.total_spent) }) }}</p>
          </div>
        </div>
        <p class="text-sm font-medium text-emerald-700 dark:text-emerald-300">
          {{ allClaimed ? t('payment.activities.allClaimed') : nextTier ? t('payment.activities.remaining', { amount: format(Math.max(0, nextTier.threshold - status.consumption_reward.total_spent)), bonus: format(nextTier.bonus) }) : '' }}
        </p>
      </div>
      <div class="mt-8 overflow-x-auto px-2 pb-3">
        <div class="relative min-w-[720px]">
          <div class="absolute left-2 right-2 top-4 h-2 rounded-full bg-gray-100 dark:bg-dark-700"><div class="h-full rounded-full bg-emerald-500 transition-all duration-500" :style="{ width: `${progress}%` }" /></div>
          <div class="relative flex justify-between gap-3">
            <div v-for="tier in status.consumption_reward.tiers" :key="tier.threshold" class="flex min-w-[92px] flex-1 flex-col items-center text-center">
              <div
                class="z-10 flex h-9 w-9 items-center justify-center rounded-full border-2 transition-all"
                :class="{
                  'border-emerald-500 bg-emerald-500 text-white': tier.claimed,
                  'border-emerald-500 bg-emerald-50 text-emerald-600 dark:bg-emerald-900/30': !tier.claimed && reached(tier),
                  'border-gray-200 bg-white text-gray-400 dark:border-dark-600 dark:bg-dark-800': !tier.claimed && !reached(tier)
                }"
              >
                <Icon :name="tier.claimed || reached(tier) ? 'check' : 'lock'" size="sm" :stroke-width="2" />
              </div>
              <span class="mt-2 rounded-full px-2 py-1 text-xs font-semibold" :class="labelClass(tier)">${{ format(tier.threshold) }} {{ t('payment.activities.giftShort', { amount: format(tier.bonus) }) }}</span>
              <button v-if="reached(tier) && !tier.claimed" type="button" class="mt-2 rounded-md bg-emerald-600 px-3 py-1 text-xs font-medium text-white transition hover:bg-emerald-700 disabled:opacity-50" :disabled="claiming === tier.threshold" @click="claim(tier.threshold)">{{ claiming === tier.threshold ? t('common.processing') : t('payment.activities.claimAction') }}</button>
              <span v-else-if="tier.claimed" class="mt-2 text-xs text-emerald-600 dark:text-emerald-300">{{ t('payment.activities.claimed') }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { paymentAPI } from '@/api/payment'
import type { RewardCampaignStatus, RewardTierStatus } from '@/types/payment'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import Icon from '@/components/icons/Icon.vue'

const emit = defineEmits<{ claimed: [] }>()
const { t } = useI18n(); const authStore = useAuthStore(); const appStore = useAppStore(); const claiming = ref<number | null>(null)
const status = ref<RewardCampaignStatus>({ recharge_bonus: { enabled: false, tiers: [] }, bonus_balance: 0, consumption_reward: { enabled: false, total_spent: 0, tiers: [] } })
const visible = computed(() => status.value.consumption_reward.enabled)
const tiers = computed(() => status.value.consumption_reward.tiers)
const nextTier = computed<RewardTierStatus | undefined>(() => tiers.value.find(tier => !tier.claimed && tier.threshold > status.value.consumption_reward.total_spent))
const allClaimed = computed(() => tiers.value.length > 0 && tiers.value.every(tier => tier.claimed))
const progress = computed(() => {
  const campaignTiers = tiers.value
  const spent = Math.max(
    0,
    Number(status.value.consumption_reward.total_spent) || 0,
    ...campaignTiers.filter(tier => tier.claimed).map(tier => tier.threshold)
  )
  const count = campaignTiers.length
  if (count === 0) return 0

  // The rail uses evenly spaced nodes, so map spending to the node positions
  // instead of scaling it against the highest dollar threshold.
  const firstPosition = 50 / count
  if (spent < campaignTiers[0].threshold) {
    return Math.min(firstPosition, spent / Math.max(campaignTiers[0].threshold, Number.EPSILON) * firstPosition)
  }

  for (let index = 0; index < count - 1; index += 1) {
    const current = campaignTiers[index]
    const next = campaignTiers[index + 1]
    if (spent < next.threshold) {
      const start = (index + 0.5) / count * 100
      const end = (index + 1.5) / count * 100
      const range = Math.max(next.threshold - current.threshold, Number.EPSILON)
      return start + Math.min(1, Math.max(0, (spent - current.threshold) / range)) * (end - start)
    }
  }

  return 100
})
function format(value: number) { return Number(value || 0).toFixed(2) }
function reached(tier: RewardTierStatus) { return status.value.consumption_reward.total_spent + 1e-9 >= tier.threshold }
function labelClass(tier: RewardTierStatus) { return tier.claimed || reached(tier) ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300' : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400' }
async function load() { status.value = (await paymentAPI.getRewardCampaigns()).data }
async function claim(threshold: number) {
  if (claiming.value !== null) return; claiming.value = threshold
  try { const tier = tiers.value.find(item => item.threshold === threshold); const response = await paymentAPI.claimConsumptionReward(threshold); if (authStore.user) authStore.user.balance = response.data.balance; await Promise.all([load(), authStore.refreshUser().catch(() => undefined)]); appStore.showSuccess(t('payment.activities.claimSuccess', { amount: `$${format(tier?.bonus ?? 0)}` })); emit('claimed') }
  catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
  finally { claiming.value = null }
}
onMounted(() => { load().catch(() => undefined) })
</script>
