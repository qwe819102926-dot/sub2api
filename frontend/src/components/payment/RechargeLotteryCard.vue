<template>
  <Teleport to="body">
    <button
      v-if="!show && status?.enabled"
      type="button"
      class="fixed bottom-6 right-4 z-40 flex w-24 flex-col items-center gap-1 rounded-2xl border border-gray-200 bg-white px-3 py-4 text-sm font-medium text-gray-700 shadow-[0_10px_28px_rgba(31,41,55,0.14)] transition-transform hover:-translate-x-1 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200"
      @click="emit('open')"
    >
      <Icon name="chevronLeft" size="sm" class="text-emerald-600" />
      <span>{{ t('payment.lottery.tab') }}</span>
      <span class="rounded-full bg-amber-50 px-2 py-0.5 text-xs font-semibold text-amber-600 dark:bg-amber-900/30 dark:text-amber-300">{{ status.remaining_draws }}</span>
    </button>
    <div v-if="show && status?.enabled" class="fixed inset-0 z-50" @click.self="emit('close')">
      <section class="absolute bottom-6 right-4 w-[min(380px,calc(100vw-2rem))] overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-[0_18px_50px_rgba(31,41,55,0.2)] dark:border-dark-600 dark:bg-dark-800">
        <div class="px-5 pb-3 pt-4">
          <div class="flex items-start justify-between">
            <div>
              <h2 class="text-xl font-bold text-gray-900 dark:text-white">{{ t('payment.lottery.title') }}</h2>
            </div>
            <div class="flex items-center gap-3">
              <span class="rounded-full bg-amber-50 px-3 py-1 text-sm font-semibold text-amber-600 dark:bg-amber-900/30 dark:text-amber-300">{{ t('payment.lottery.chances', { count: status.remaining_draws }) }}</span>
              <button type="button" class="text-sm text-gray-400 hover:text-gray-700 dark:hover:text-gray-200" @click="emit('close')">{{ t('payment.lottery.collapse') }}</button>
            </div>
          </div>
        </div>
        <div class="px-5 pb-5">
          <div class="grid grid-cols-3 gap-2 rounded-xl bg-gray-50 p-2 dark:bg-dark-900" aria-live="polite">
            <div
              v-for="(prize, index) in drawPrizes"
              :key="`${prize.amount}-${prize.probability}`"
              class="lottery-prize rounded-lg border border-gray-200 bg-white px-2 py-4 text-center shadow-sm dark:border-dark-600 dark:bg-dark-800"
              :class="{ 'lottery-prize--active': activePrizeIndex === index }"
            >
              <p class="text-xl font-bold text-emerald-600 dark:text-emerald-400">${{ prize.amount.toFixed(2) }}</p>
            </div>
          </div>
          <button type="button" class="mt-5 w-full rounded-xl bg-gray-600 px-4 py-3 text-base font-medium text-white transition-colors hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-50" :disabled="status.remaining_draws <= 0 || drawing" @click="draw">
            <span v-if="drawing">{{ t('common.processing') }}</span>
            <span v-else>{{ t('payment.lottery.drawNow') }}</span>
          </button>
          <p v-if="resultMessage" class="mt-3 text-center text-sm font-medium" :class="lastResult?.is_winner ? 'text-emerald-600' : 'text-gray-500 dark:text-gray-400'">{{ resultMessage }}</p>
          <p class="mt-4 text-xs text-gray-500 dark:text-gray-400">{{ t('payment.lottery.rule', { amount: status.threshold.toFixed(2) }) }}</p>
          <p class="mt-2 text-xs text-gray-400 dark:text-gray-500">{{ t('payment.lottery.openHint') }}</p>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { paymentAPI } from '@/api/payment'
import type { RechargeLotteryDrawResult, RechargeLotteryStatus } from '@/types/payment'
import { useAuthStore } from '@/stores/auth'
import Icon from '@/components/icons/Icon.vue'
withDefaults(defineProps<{ show: boolean }>(), { show: false })
const emit = defineEmits<{ available: []; close: []; open: []; drawn: [] }>()

const { t } = useI18n()
const authStore = useAuthStore()
const status = ref<RechargeLotteryStatus | null>(null)
const drawing = ref(false)
const activePrizeIndex = ref<number | null>(null)
const lastResult = ref<RechargeLotteryDrawResult | null>(null)

const drawPrizes = computed(() => [...(status.value?.prizes ?? [])].sort((a, b) => a.amount - b.amount))

const resultMessage = computed(() => {
  if (!lastResult.value) return ''
  return lastResult.value.is_winner ? t('payment.lottery.won', { amount: lastResult.value.prize_amount.toFixed(2) }) : ''
})

async function load() {
  const response = await paymentAPI.getRechargeLottery()
  status.value = response.data
  if (status.value.enabled) emit('available')
}

async function draw() {
  if (!status.value || drawing.value || status.value.remaining_draws <= 0) return
  drawing.value = true
  lastResult.value = null
  try {
    const response = await playDrawAnimation(paymentAPI.drawRechargeLottery())
    status.value.remaining_draws = response.remaining_draws
    lastResult.value = response
    if (response.is_winner) {
      // The response contains the unchanged principal balance. Refresh the
      // dashboard so its separate promotional-balance card updates too.
      if (authStore.user && Number.isFinite(response.balance)) {
        authStore.user.balance = response.balance
      }
      await authStore.refreshUser().catch(() => undefined)
      emit('drawn')
    }
  } finally {
    drawing.value = false
  }
}

function wait(ms: number) {
  return new Promise<void>(resolve => window.setTimeout(resolve, ms))
}

async function playDrawAnimation(resultPromise: Promise<{ data: RechargeLotteryDrawResult }>) {
  const prizes = drawPrizes.value
  activePrizeIndex.value = 0
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const fastDelay = reducedMotion ? 20 : 120
  const slowDelays = reducedMotion ? [20, 20, 20] : [200, 280, 350, 500]

  if (!prizes.length) return (await resultPromise).data

  let requestError: unknown
  let requestSettled = false
  resultPromise.then(_value => {
    requestSettled = true
  }, error => {
    requestError = error
    requestSettled = true
  })

  // Keep cycling in the fixed 1 -> 2 -> 5 order while the draw is committed.
  while (!requestSettled) {
    for (let index = 0; index < prizes.length; index += 1) {
      activePrizeIndex.value = index
      await wait(fastDelay)
      if (requestSettled) break
    }
  }
  if (requestError) throw requestError
  const drawResult = (await resultPromise).data

  const targetIndex = drawResult.is_winner
    ? Math.max(0, prizes.findIndex(prize => prize.amount === drawResult.prize_amount))
    : activePrizeIndex.value ?? 0
  let step = 0
  const currentIndex = activePrizeIndex.value ?? 0
  const distance = (targetIndex - currentIndex + prizes.length) % prizes.length
  const stepsToTarget = distance === 0 ? prizes.length : distance
  while (step < stepsToTarget) {
    await wait(slowDelays[Math.min(step, slowDelays.length - 1)])
    activePrizeIndex.value = ((activePrizeIndex.value ?? 0) + 1) % prizes.length
    step += 1
  }
  await wait(reducedMotion ? 20 : 600)
  return drawResult
}

onMounted(() => { load().catch(() => {}) })
</script>

<style scoped>
.lottery-prize {
  box-sizing: border-box;
  transform: scale(1);
  transition: transform 120ms ease, border-color 120ms ease, background-color 120ms ease, box-shadow 120ms ease;
}

.lottery-prize--active {
  border: 2px solid #f5b82e;
  background-color: #fffaf0;
  box-shadow: 0 0 0 2px rgba(245, 184, 46, 0.16), 0 8px 18px rgba(245, 184, 46, 0.2);
  transform: scale(1.05);
}

@media (prefers-reduced-motion: reduce) {
  .lottery-prize {
    transition-duration: 0.01ms;
  }
}
</style>
