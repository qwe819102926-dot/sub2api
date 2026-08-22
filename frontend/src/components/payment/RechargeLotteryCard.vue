<template>
  <section v-if="status?.enabled" class="overflow-hidden rounded-lg border border-[#c9dfd2] bg-[#faf5eb] shadow-[0_8px_24px_rgba(87,116,91,0.12)] dark:border-[#4a6a5d] dark:bg-dark-800">
    <div class="flex items-center justify-between border-b border-[#dce9df] bg-[#eff7ef] px-5 py-3 dark:border-dark-700 dark:bg-dark-700">
      <div class="flex items-center gap-2 text-sm font-semibold text-[#426b5c] dark:text-[#a8d5ba]">
        <Icon name="gift" size="sm" />
        <span>{{ t('payment.lottery.title') }}</span>
      </div>
      <span class="rounded-full bg-white px-2.5 py-1 text-xs font-semibold text-[#426b5c] shadow-sm dark:bg-dark-800 dark:text-[#a8d5ba]">
        {{ t('payment.lottery.chances', { count: status.remaining_draws }) }}
      </span>
    </div>
    <div class="p-5">
      <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
        <div v-for="prize in status.prizes" :key="`${prize.amount}-${prize.probability}`" class="rounded-md border border-[#d4e5d6] bg-white px-3 py-3 text-center dark:border-dark-600 dark:bg-dark-900">
          <p class="text-base font-bold text-[#4f8a72]">${{ prize.amount.toFixed(2) }}</p>
          <p class="mt-0.5 text-[11px] text-gray-400">{{ prize.probability }}%</p>
        </div>
      </div>
      <button type="button" class="mt-4 w-full rounded-md bg-[#639d8c] px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-[#527f72] disabled:cursor-not-allowed disabled:opacity-50" :disabled="status.remaining_draws <= 0 || drawing" @click="draw">
        <span v-if="drawing">{{ t('common.processing') }}</span>
        <span v-else>{{ t('payment.lottery.drawNow') }}</span>
      </button>
      <p v-if="resultMessage" class="mt-3 text-center text-sm font-medium" :class="lastResult?.is_winner ? 'text-[#4f8a72]' : 'text-gray-500 dark:text-gray-400'">{{ resultMessage }}</p>
      <p class="mt-3 text-center text-xs text-gray-500 dark:text-gray-400">{{ t('payment.lottery.rule', { amount: status.threshold.toFixed(2) }) }}</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { paymentAPI } from '@/api/payment'
import type { RechargeLotteryDrawResult, RechargeLotteryStatus } from '@/types/payment'
import { useAuthStore } from '@/stores/auth'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const authStore = useAuthStore()
const status = ref<RechargeLotteryStatus | null>(null)
const drawing = ref(false)
const lastResult = ref<RechargeLotteryDrawResult | null>(null)

const resultMessage = computed(() => {
  if (!lastResult.value) return ''
  return lastResult.value.is_winner
    ? t('payment.lottery.won', { amount: lastResult.value.prize_amount.toFixed(2) })
    : t('payment.lottery.notWon')
})

async function load() {
  const response = await paymentAPI.getRechargeLottery()
  status.value = response.data
}

async function draw() {
  if (!status.value || drawing.value || status.value.remaining_draws <= 0) return
  drawing.value = true
  try {
    const response = await paymentAPI.drawRechargeLottery()
    lastResult.value = response.data
    status.value.remaining_draws = response.data.remaining_draws
    if (response.data.is_winner) await authStore.refreshUser()
  } finally {
    drawing.value = false
  }
}

onMounted(() => { load().catch(() => {}) })
</script>
