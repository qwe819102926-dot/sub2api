<template>
  <section v-if="compact && !show" class="rounded-xl border border-amber-100 bg-white p-5 shadow-sm dark:border-amber-900/40 dark:bg-dark-800">
      <div class="flex flex-wrap items-center justify-between gap-4"><div class="flex items-center gap-3"><div class="rounded-xl bg-amber-100 p-2 text-amber-600"><Icon name="calendar" size="md" /></div><div><h2 class="font-semibold text-gray-900 dark:text-white">{{ t('payment.checkin.title') }}</h2><p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ status?.checked_in_today ? t('payment.checkin.checkedToday') : t('payment.checkin.todayReward', { amount: format(status?.today_reward ?? 0) }) }}</p></div></div><div class="flex items-center gap-4"><div class="text-right"><p class="text-xs text-gray-500">{{ t('payment.checkin.currentStreak') }}</p><p class="font-bold text-amber-600">{{ status?.current_streak ?? 0 }} {{ t('payment.checkin.days') }}</p></div><button type="button" class="rounded-lg border border-amber-200 px-3 py-2 text-sm font-medium text-amber-700 hover:bg-amber-50 dark:border-amber-800 dark:text-amber-300 dark:hover:bg-amber-900/20" @click="open">{{ t('payment.checkin.viewDetails') }}</button></div></div>
    </section>
  <Teleport v-else to="body">
    <button v-if="floating && !show" type="button" class="fixed bottom-24 right-4 z-40 flex w-24 flex-col items-center gap-1 rounded-2xl border border-amber-200 bg-white px-3 py-4 text-sm font-medium text-gray-700 shadow-lg transition hover:-translate-x-1 dark:border-amber-800 dark:bg-dark-800 dark:text-gray-200" @click="open">
      <Icon name="calendar" size="sm" class="text-amber-500" />
      <span>{{ t('payment.checkin.title') }}</span>
      <span class="text-xs text-emerald-600">{{ status?.checked_in_today ? t('payment.checkin.checked') : t('payment.checkin.available') }}</span>
    </button>
    <div v-if="show" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/45 p-4" @click.self="close">
      <section class="w-[min(760px,100%)] max-h-[90vh] overflow-auto rounded-2xl border border-amber-100 bg-white shadow-2xl dark:border-dark-600 dark:bg-dark-800">
        <header class="flex items-start justify-between border-b border-amber-100 bg-amber-50/70 px-6 py-5 dark:border-dark-700 dark:bg-amber-900/10">
          <div class="flex items-center gap-3"><div class="rounded-xl bg-amber-100 p-2 text-amber-600"><Icon name="calendar" size="lg" /></div><div><h2 class="text-xl font-bold text-gray-900 dark:text-white">{{ t('payment.checkin.title') }}</h2><p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.checkin.subtitle') }}</p></div></div>
          <button type="button" class="text-sm text-gray-500 hover:text-gray-900 dark:hover:text-white" @click="close">{{ t('common.close') }}</button>
        </header>
        <div v-if="loading" class="p-10 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
        <div v-else-if="status" class="space-y-5 p-6">
          <div class="grid grid-cols-3 gap-3"><div class="rounded-xl border border-gray-200 p-4 dark:border-dark-600"><p class="text-xs text-gray-500">{{ t('payment.checkin.currentStreak') }}</p><p class="mt-1 text-2xl font-bold text-amber-600">{{ status.current_streak }}<span class="ml-1 text-sm font-normal">{{ t('payment.checkin.days') }}</span></p></div><div class="rounded-xl border border-gray-200 p-4 dark:border-dark-600"><p class="text-xs text-gray-500">{{ t('payment.checkin.totalDays') }}</p><p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ status.total_days }}</p></div><div class="rounded-xl border border-gray-200 p-4 dark:border-dark-600"><p class="text-xs text-gray-500">{{ t('payment.checkin.totalReward') }}</p><p class="mt-1 text-2xl font-bold text-emerald-600">+{{ format(status.total_reward) }}</p></div></div>
          <div class="rounded-xl bg-emerald-50 p-4 dark:bg-emerald-900/15"><div class="flex flex-wrap items-center justify-between gap-3"><div><p class="font-semibold text-gray-900 dark:text-white">{{ status.checked_in_today ? t('payment.checkin.checkedToday') : t('payment.checkin.todayReward', { amount: format(status.today_reward) }) }}</p><p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('payment.checkin.bonusOnly') }}</p></div><button type="button" class="rounded-lg bg-emerald-600 px-4 py-2 text-sm font-semibold text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-50" :disabled="status.checked_in_today || submitting" @click="checkin">{{ status.checked_in_today ? t('payment.checkin.checked') : submitting ? t('common.processing') : t('payment.checkin.checkinNow') }}</button></div></div>
          <div><div class="mb-3 flex items-center justify-between"><h3 class="font-semibold text-gray-900 dark:text-white">{{ status.month }} {{ t('payment.checkin.calendar') }}</h3><span class="text-xs text-gray-500">{{ t('payment.checkin.cycleHint', { days: status.cycle_length }) }}</span></div><div class="grid grid-cols-7 gap-2 text-center text-xs text-gray-500"><div v-for="weekday in weekdayLabels" :key="weekday">{{ weekday }}</div></div><div class="mt-2 grid grid-cols-7 gap-2"><div v-for="(day, index) in monthDays" :key="day ?? `empty-${index}`" class="min-h-20 rounded-lg border p-2 text-left" :class="day ? (recordByDate[day] ? 'border-emerald-200 bg-emerald-50/80 dark:border-emerald-800 dark:bg-emerald-900/10' : 'border-gray-200 dark:border-dark-600') : 'border-transparent'"> <template v-if="day"><p class="text-xs text-gray-500">{{ day.slice(-2) }}</p><p v-if="recordByDate[day]" class="mt-2 text-sm font-semibold text-emerald-700 dark:text-emerald-300">+{{ format(recordByDate[day].reward) }}</p></template></div></div></div>
          <div><h3 class="mb-3 font-semibold text-gray-900 dark:text-white">{{ t('payment.checkin.rewardSchedule') }}</h3><div class="grid grid-cols-4 gap-2 sm:grid-cols-7"><div v-for="(reward, index) in status.reward_cycle" :key="index" class="rounded-lg border border-amber-100 bg-amber-50/60 p-3 text-center dark:border-amber-900/40 dark:bg-amber-900/10"><p class="text-xs text-gray-500">{{ t('payment.checkin.dayNumber', { day: index + 1 }) }}</p><p class="mt-1 font-bold text-amber-700 dark:text-amber-300">+{{ format(reward) }}</p></div></div></div>
          <div><h3 class="mb-3 font-semibold text-gray-900 dark:text-white">{{ t('payment.checkin.details') }}</h3><div class="divide-y divide-gray-100 rounded-xl border border-gray-200 dark:divide-dark-700 dark:border-dark-600"><div v-for="record in recentRecords" :key="record.date" class="flex items-center justify-between px-4 py-3 text-sm"><div><p class="font-medium text-gray-800 dark:text-gray-200">{{ record.date }}</p><p class="text-xs text-gray-500">{{ t('payment.checkin.streakDay', { day: record.streak_day }) }}</p></div><strong class="text-emerald-600">+{{ format(record.reward) }}</strong></div><p v-if="recentRecords.length === 0" class="p-5 text-center text-sm text-gray-500">{{ t('payment.checkin.noRecords') }}</p></div></div>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { paymentAPI } from '@/api/payment'
import type { DailyCheckinRecord, DailyCheckinStatus } from '@/types/payment'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{ show?: boolean; compact?: boolean; floating?: boolean }>(), { show: false, compact: false, floating: true })
const emit = defineEmits<{ 'update:show': [boolean]; checked: [] }>()
const { t } = useI18n(); const authStore = useAuthStore(); const appStore = useAppStore()
const show = ref(props.show); const compact = props.compact; const floating = props.floating; const loading = ref(false); const submitting = ref(false); const status = ref<DailyCheckinStatus | null>(null)
const monthDays = computed<(string | null)[]>(() => { if (!status.value) return []; const [year, month] = status.value.month.split('-').map(Number); const days = new Date(year, month, 0).getDate(); const firstWeekday = new Date(year, month - 1, 1).getDay(); return [...Array<string | null>(firstWeekday).fill(null), ...Array.from({ length: days }, (_, i) => `${status.value!.month}-${String(i + 1).padStart(2, '0')}`)] })
const weekdayLabels = computed(() => String(t('payment.checkin.weekdays')).split(','))
const recordByDate = computed<Record<string, DailyCheckinRecord>>(() => Object.fromEntries((status.value?.records ?? []).map(record => [record.date, record])))
const recentRecords = computed(() => [...(status.value?.records ?? [])].reverse().slice(0, 12))
function format(value: number) { return Number(value || 0).toFixed(2) }
function open() { show.value = true; emit('update:show', true); load().catch(() => appStore.showError(t('payment.checkin.loadFailed'))) }
function close() { show.value = false; emit('update:show', false) }
async function load() { loading.value = true; try { status.value = (await paymentAPI.getDailyCheckin()).data } finally { loading.value = false } }
async function checkin() { if (!status.value || submitting.value || status.value.checked_in_today) return; submitting.value = true; try { status.value = (await paymentAPI.claimDailyCheckin()).data; await authStore.refreshUser(); appStore.showSuccess(t('payment.checkin.success')); emit('checked') } catch { appStore.showError(t('payment.checkin.failed')); await load().catch(() => undefined) } finally { submitting.value = false } }
onMounted(() => { if (compact || floating || props.show) load().catch(() => appStore.showError(t('payment.checkin.loadFailed'))) })
watch(() => props.show, value => {
  show.value = value
  if (value) load().catch(() => appStore.showError(t('payment.checkin.loadFailed')))
})
</script>
