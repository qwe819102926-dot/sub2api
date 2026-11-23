<template>
  <button ref="trigger" type="button" class="ci-trigger" :class="{ checked: status?.checked_in_today }" aria-haspopup="dialog" :aria-expanded="isOpen" :aria-label="t(status?.checked_in_today ? 'checkin.checkedEntry' : 'checkin.title')" @click="open">
    <Icon :name="status?.checked_in_today ? 'checkCircle' : 'calendar'" size="sm" /><span class="hidden sm:inline">{{ t(status?.checked_in_today ? 'checkin.checkedEntry' : 'checkin.title') }}</span>
  </button>
  <Teleport to="body">
    <div v-if="isOpen" class="ci-overlay" @click.self="close">
      <section ref="dialog" class="ci-dialog" :class="{ expanded: showDetails }" role="dialog" aria-modal="true" :aria-labelledby="titleId" :aria-busy="loading || submitting">
        <header class="ci-header">
          <span class="ci-header-icon"><Icon name="calendar" size="lg" /></span>
          <h2 :id="titleId">{{ t('checkin.title') }}</h2>
          <button ref="closeButton" type="button" class="ci-close" :aria-label="t('common.close')" @click="close"><Icon name="x" size="md" /></button>
        </header>
        <div class="ci-body">
          <div v-if="!status" class="ci-empty" role="status">
            <template v-if="loading">{{ t('common.loading') }}</template>
            <template v-else><p>{{ t('checkin.loadFailed') }}</p><button class="ci-action" type="button" @click="load()">{{ t('checkin.retry') }}</button></template>
          </div>
          <template v-else>
            <section class="ci-claim" aria-live="polite">
              <div v-if="!status.checked_in_today && !status.has_recent_recharge && !status.recharge_progress" class="ci-callout locked">
                <span class="ci-callout-icon"><Icon name="lock" size="md" /></span>
                <div>
                  <h3>{{ t('checkin.rechargeToUnlock') }}</h3>
                  <p>{{ t('checkin.rechargeRequired') }}</p>
                  <p class="ci-muted">{{ t('checkin.rechargeGiftExcluded') }}</p>
                </div>
              </div>
              <div v-else-if="status.checked_in_today || status.has_recent_recharge" class="ci-callout">
                <span class="ci-callout-icon"><Icon :name="status.checked_in_today ? 'checkCircle' : 'gift'" size="md" /></span>
                <div>
                  <h3>{{ t(status.checked_in_today ? 'checkin.checkedToday' : 'checkin.available') }}</h3>
                  <p v-if="todayRewardLabel" class="ci-today-reward">{{ todayRewardLabel }}<small>{{ t('checkin.bonusUnit') }}</small></p>
                </div>
              </div>
              <dl class="ci-stats">
                <div><dt>{{ t('checkin.totalDays') }}</dt><dd>{{ status.total_days }}<small>{{ t('checkin.times') }}</small></dd></div>
                <div><dt>{{ t('checkin.totalReward') }}</dt><dd>{{ format(status.total_reward) }}<small>{{ t('checkin.bonusUnit') }}</small></dd></div>
              </dl>
              <section v-if="status.recharge_progress" class="ci-recharge" :aria-labelledby="`${titleId}-recharge`">
                <header class="ci-recharge-heading">
                  <h3 :id="`${titleId}-recharge`"><Icon name="chartBar" size="sm" />{{ t('checkin.rechargeProgress') }}</h3>
                  <span class="ci-recharge-badge" :class="{ met: status.has_recent_recharge }">
                    {{ t(status.has_recent_recharge ? 'checkin.rechargeMet' : 'checkin.rechargeUnmet') }}
                  </span>
                </header>
                <dl class="ci-recharge-amounts">
                  <div><dt>{{ t('checkin.rechargePaid') }}</dt><dd>{{ formatRecharge(status.recharge_progress.total) }}</dd></div>
                  <div><dt>{{ t('checkin.rechargeTarget') }}</dt><dd>{{ formatRecharge(status.recharge_progress.target) }}</dd></div>
                </dl>
                <progress
                  class="ci-recharge-meter"
                  :aria-labelledby="`${titleId}-recharge`"
                  :value="Math.min(status.recharge_progress.total, status.recharge_progress.target)"
                  :max="status.recharge_progress.target"
                />
                <p v-if="status.recharge_progress.remaining > 0" class="ci-recharge-gap">
                  {{ t('checkin.rechargeGap') }} <strong>{{ formatRecharge(status.recharge_progress.remaining) }}</strong>
                </p>
                <p class="ci-recharge-note">{{ t('checkin.rechargeGiftExcluded') }}</p>
              </section>
              <button type="button" class="ci-action" :disabled="loading || submitting || status.checked_in_today || !status.has_recent_recharge" @click="claim">
                {{ t(status.checked_in_today ? 'checkin.checkedEntry' : submitting ? 'common.processing' : status.has_recent_recharge ? 'checkin.checkinNow' : 'checkin.rechargeUnmet') }}
              </button>
              <p class="ci-note">{{ t('checkin.dailyNote') }}</p>
            </section>
          </template>

          <section class="ci-trial">
            <Icon class="ci-trial-icon" name="gift" size="lg" />
            <div>
              <h3>{{ t('checkin.trialTitle') }}</h3>
              <p>{{ t('checkin.trialJoinGroup') }}</p>
            </div>
            <a class="ci-trial-qr" :href="trialGroupQR" target="_blank" rel="noopener" :aria-label="t('checkin.trialOpenQR')">
              <img :src="trialGroupQR" :alt="t('checkin.trialQR')" width="305" height="295" />
            </a>
          </section>

          <footer v-if="status" class="ci-footer">
            <button type="button" class="ci-link" :aria-expanded="showDetails" @click="showDetails = !showDetails">{{ t('checkin.historyAndRules') }}<Icon :name="showDetails ? 'chevronUp' : 'chevronDown'" size="xs" /></button>
          </footer>
          <template v-if="status && showDetails">
            <p class="ci-muted">{{ t('checkin.currentStreak') }} {{ status.current_streak }} {{ t('checkin.days') }}</p>
            <div class="ci-columns">
              <section class="ci-calendar" :aria-label="t('checkin.calendar')">
                <div class="ci-calendar-heading"><h3>{{ t('checkin.calendar') }}</h3><div class="ci-month-nav"><button type="button" :aria-label="t('checkin.previousMonth')" :disabled="loading || submitting" @click="changeMonth(-1)"><Icon name="chevronLeft" size="sm" /></button><strong>{{ monthLabel }}</strong><button type="button" :aria-label="t('checkin.nextMonth')" :disabled="loading || submitting" @click="changeMonth(1)"><Icon name="chevronRight" size="sm" /></button></div></div>
                <p class="ci-hint">{{ t('checkin.calendarHint') }}</p>
                <div class="ci-grid"><span v-for="weekday in weekdays" :key="weekday" class="ci-weekday">{{ weekday }}</span><template v-for="(day, index) in calendarDays" :key="day?.date || `blank-${index}`">
                  <button v-if="day" type="button" class="ci-date" :class="{ signed: day.record, today: day.date === status.today, node: day.milestone, selected: day.date === selectedDate }" :aria-label="dayDescription(day.date)" :aria-current="day.date === status.today ? 'date' : undefined" :aria-pressed="day.date === selectedDate" :title="dayDescription(day.date)" @click="selectedDate = day.date">
                    <span>{{ Number(day.date.slice(-2)) }}</span><small v-if="day.milestone" class="ci-node-label">{{ t('checkin.nodeDay', { day: day.milestone.day }) }}</small><Icon v-else-if="day.record" name="checkCircle" size="xs" class="ci-date-check" /><small v-else-if="day.date === status.today" class="ci-node-label">{{ t('checkin.today') }}</small>
                    <strong v-if="day.record || day.milestone" class="ci-date-reward">+{{ format(day.record?.reward ?? day.milestone?.reward ?? 0) }}</strong><span v-if="day.record" class="ci-stamp" aria-hidden="true">{{ t('checkin.checked') }}</span>
                  </button><span v-else aria-hidden="true" class="ci-blank"></span>
                </template></div>
                <div class="ci-legend"><span><i class="signed"></i>{{ t('checkin.checked') }}</span><span><i class="today"></i>{{ t('checkin.today') }}</span><span><i class="node"></i>{{ t('checkin.milestone') }}</span></div><p class="ci-selection" aria-live="polite">{{ dayDescription(selectedDate) }}</p>
                <button v-if="status.month !== status.today.slice(0, 7)" type="button" class="ci-link" :disabled="loading || submitting" @click="load(status.today.slice(0, 7))">{{ t('checkin.backToToday') }}</button>
              </section>
              <aside>
                <section class="ci-stage"><div class="ci-stage-heading"><h3><Icon name="trophy" size="sm" />{{ t('checkin.stages') }}</h3><span>{{ completedInCycle }} / {{ status.cycle_length }} {{ t('checkin.days') }}</span></div><p class="ci-muted">{{ t('checkin.stageHint') }}</p><div class="ci-milestones"><button v-for="milestone in milestones" :key="milestone.day" type="button" class="ci-milestone" :class="{ complete: milestone.day <= completedInCycle, next: milestone.day === nextMilestone?.day }" :disabled="loading || submitting" :aria-label="`${t('checkin.streakDay', { day: milestone.day })}, ${t('checkin.dayTotal')}, +${format(milestone.reward)}, ${milestone.date}`" @click="selectMilestone(milestone.date)"><span>{{ t('checkin.nodeDay', { day: milestone.day }) }}<Icon name="gift" size="xs" /></span><strong>+{{ format(milestone.reward) }}</strong><small>{{ t('checkin.dayTotal') }}</small><small>{{ milestone.day <= completedInCycle ? t('checkin.reached') : t('checkin.remaining', { days: milestone.day - completedInCycle }) }}</small></button></div></section>
                <section class="ci-records"><div><h3>{{ t('checkin.recentRecords') }}</h3><button v-if="status.records.length > 3" type="button" class="ci-link" :aria-expanded="allRecords" @click="allRecords = !allRecords">{{ t(allRecords ? 'checkin.lessRecords' : 'checkin.allRecords') }}</button></div><div v-for="record in recentRecords" :key="record.date" class="ci-record"><span>{{ record.date }}<small>{{ t('checkin.streakDay', { day: record.streak_day }) }}</small></span><strong>+{{ format(record.reward) }}</strong></div><p v-if="!recentRecords.length" class="ci-muted">{{ t('checkin.noRecords') }}</p></section>
              </aside>
            </div>
            <div class="ci-rules"><p>{{ t('checkin.rechargeRequired') }} {{ t('checkin.rechargeGiftExcluded') }}</p><p>{{ t('checkin.ordinaryRule', { rewards: status.ordinary_rewards.map(format).join('～') }) }}</p><p>{{ t('checkin.milestoneRule') }}</p><p>{{ t('checkin.resetRule', { days: status.cycle_length }) }}</p></div>
          </template>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { paymentAPI } from '@/api/payment'
import type { DailyCheckinStatus } from '@/types/payment'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import Icon from '@/components/icons/Icon.vue'
import trialGroupQR from '@/assets/trial-credit-group-qq.png'

const props = withDefaults(defineProps<{ show?: boolean }>(), { show: false })
const emit = defineEmits<{ 'update:show': [boolean]; checked: [] }>()
const { t, locale } = useI18n()
const auth = useAuthStore()
const app = useAppStore()
const isOpen = ref(props.show)
const status = ref<DailyCheckinStatus | null>(null)
const loading = ref(false)
const submitting = ref(false)
const allRecords = ref(false)
const showDetails = ref(false)
const selectedDate = ref('')
const trigger = ref<HTMLButtonElement | null>(null)
const closeButton = ref<HTMLButtonElement | null>(null)
const dialog = ref<HTMLElement | null>(null)
const titleId = `daily-checkin-${auth.user?.id ?? 'user'}`
const dayMs = 86400000
let active = true
let revision = 0
let resetTimer: ReturnType<typeof setTimeout> | undefined
let lastLoaded = 0
const dateValue = (value: string) => Date.parse(`${value}T00:00:00Z`)
const dateString = (value: number) => new Date(value).toISOString().slice(0, 10)
const format = (value: number) => {
  const amount = Number(value || 0)
  return Number.isInteger(amount) ? String(amount) : amount.toFixed(2)
}
const rechargeFormatter = computed(() => new Intl.NumberFormat(locale.value, {
  style: 'currency', currency: 'CNY', minimumFractionDigits: 0, maximumFractionDigits: 2
}))
const formatRecharge = (value: number) => rechargeFormatter.value.format(value)
const weekdays = computed(() => String(t('checkin.weekdays')).split(','))
const records = computed(() => Object.fromEntries((status.value?.records ?? []).map(record => [record.date, record])))
const recentRecords = computed(() => [...(status.value?.records ?? [])].reverse().slice(0, allRecords.value ? undefined : 3))
const monthLabel = computed(() => status.value ? new Intl.DateTimeFormat(locale.value, { year: 'numeric', month: 'long', timeZone: 'UTC' }).format(new Date(`${status.value.month}-01T00:00:00Z`)) : '')
const completedInCycle = computed(() => status.value ? status.value.cycle_day - (status.value.checked_in_today ? 0 : 1) : 0)
const milestones = computed(() => status.value?.milestones.map(milestone => ({ ...milestone, date: dateString(dateValue(status.value!.today) + (milestone.day - status.value!.cycle_day) * dayMs) })) ?? [])
const nextMilestone = computed(() => milestones.value.find(milestone => milestone.day > completedInCycle.value))
const todayRewardLabel = computed(() => {
  const current = status.value
  if (!current) return ''
  if (!current.checked_in_today && !current.milestones.some(milestone => milestone.day === current.cycle_day)) {
    return ''
  }
  return `+${format(current.today_reward)}`
})
function milestoneOn(date: string) {
  const current = status.value
  if (!current) return undefined
  const record = records.value[date]
  if (record) return current.milestones.find(milestone => milestone.day === (record.streak_day - 1) % current.cycle_length + 1 && milestone.reward === record.reward)
  return date >= current.today ? milestones.value.find(milestone => milestone.date === date) : undefined
}
const calendarDays = computed(() => {
  if (!status.value) return []
  const [year, month] = status.value.month.split('-').map(Number)
  const offset = (new Date(Date.UTC(year, month - 1, 1)).getUTCDay() + 6) % 7
  const count = new Date(Date.UTC(year, month, 0)).getUTCDate()
  return [...Array<null>(offset).fill(null), ...Array.from({ length: count }, (_, i) => {
    const date = `${status.value!.month}-${String(i + 1).padStart(2, '0')}`
    return { date, record: records.value[date], milestone: milestoneOn(date) }
  })]
})
function dayDescription(date: string) {
  if (!date || !status.value) return ''
  const record = records.value[date]
  if (record) return `${date} · ${t('checkin.streakDay', { day: record.streak_day })} · +${format(record.reward)}`
  const milestone = milestoneOn(date)
  if (milestone) return `${date} · ${t('checkin.projectedNode', { day: milestone.day })} · +${format(milestone.reward)}`
  return `${date} · ${t(date === status.value.today ? 'checkin.today' : date > status.value.today ? 'checkin.pending' : 'checkin.noRecord')}`
}
function scheduleReset() {
  clearTimeout(resetTimer)
  if (!status.value) return
  const remaining = Date.parse(status.value.next_reset_at) - Date.now()
  resetTimer = setTimeout(() => { void load() }, Math.min(Math.max(remaining + 1000, 1000), 86401000))
}
async function load(month?: string, notify = true) {
  if (!auth.user || submitting.value) return
  const owner = auth.user.id
  const request = ++revision
  loading.value = true
  try {
    const { data } = await paymentAPI.getDailyCheckin(month)
    if (!active || request !== revision || auth.user?.id !== owner) return
    status.value = data
    if (!selectedDate.value || !selectedDate.value.startsWith(data.month)) selectedDate.value = data.month === data.today.slice(0, 7) ? data.today : `${data.month}-01`
    lastLoaded = Date.now()
    scheduleReset()
  } catch {
    if (active && request === revision && notify) app.showError(t('checkin.loadFailed'))
    if (active && request === revision) { clearTimeout(resetTimer); resetTimer = setTimeout(() => { void load(undefined, false) }, 60000) }
  } finally { if (active && request === revision) loading.value = false }
}
function open() { isOpen.value = true; emit('update:show', true); void load(status.value?.month) }
function close() { isOpen.value = false; emit('update:show', false) }
async function changeMonth(offset: number) {
  if (!status.value) return
  const [year, month] = status.value.month.split('-').map(Number)
  allRecords.value = false
  await load(dateString(Date.UTC(year, month - 1 + offset, 1)).slice(0, 7))
}
async function selectMilestone(date: string) { selectedDate.value = date; if (status.value?.month !== date.slice(0, 7)) await load(date.slice(0, 7)) }
async function claim() {
  if (!status.value?.has_recent_recharge || loading.value || submitting.value || status.value.checked_in_today || !auth.user) return
  const owner = auth.user.id
  const request = ++revision
  submitting.value = true
  try {
    const { data } = await paymentAPI.claimDailyCheckin()
    if (!active || request !== revision || auth.user?.id !== owner) return
    status.value = data
    selectedDate.value = data.today
    scheduleReset()
    app.showSuccess(t('checkin.success'))
    emit('checked')
    await auth.refreshUser().catch(() => undefined)
  } catch (error) {
    if (active && request === revision && auth.user?.id === owner) {
      const rechargeRequired = typeof error === 'object' && error !== null && 'reason' in error && error.reason === 'CHECKIN_RECHARGE_REQUIRED'
      if (rechargeRequired && status.value) {
        status.value.has_recent_recharge = false
        status.value.recharge_progress = undefined
      }
      app.showError(t(rechargeRequired ? 'checkin.rechargeRequired' : 'checkin.failed'))
    }
  }
  finally { if (active && request === revision) { submitting.value = false; loading.value = false; void load(status.value?.month, false) } }
}
function refreshOnFocus() { if (document.visibilityState !== 'hidden' && Date.now() - lastLoaded > 60000) void load(status.value?.month, false) }
function handleKey(event: KeyboardEvent) {
  if (!isOpen.value) return
  if (event.key === 'Escape') { event.preventDefault(); close(); return }
  if (event.key !== 'Tab' || !dialog.value) return
  const buttons = [...dialog.value.querySelectorAll<HTMLElement>('button:not(:disabled), [href], input:not(:disabled), [tabindex="0"]')].filter(element => element.getClientRects().length > 0)
  const first = buttons[0]; const last = buttons[buttons.length - 1]
  if (event.shiftKey && (document.activeElement === first || !dialog.value.contains(document.activeElement))) { event.preventDefault(); last?.focus() }
  else if (!event.shiftKey && (document.activeElement === last || !dialog.value.contains(document.activeElement))) { event.preventDefault(); first?.focus() }
}
watch(() => props.show, value => { if (value) open(); else isOpen.value = false })
watch(isOpen, async (value, previous) => {
  if (value) { document.body.classList.add('modal-open'); await nextTick(); if (isOpen.value) closeButton.value?.focus() }
  else { await nextTick(); if (!document.querySelector('[aria-modal="true"]')) document.body.classList.remove('modal-open'); if (previous) trigger.value?.focus() }
}, { immediate: true })
onMounted(() => { void load(undefined, false); window.addEventListener('focus', refreshOnFocus); document.addEventListener('visibilitychange', refreshOnFocus); document.addEventListener('keydown', handleKey) })
onBeforeUnmount(() => { active = false; revision++; clearTimeout(resetTimer); window.removeEventListener('focus', refreshOnFocus); document.removeEventListener('visibilitychange', refreshOnFocus); document.removeEventListener('keydown', handleKey); if (!document.querySelector('[aria-modal="true"]:not(.ci-dialog)')) document.body.classList.remove('modal-open') })
</script>

<style scoped>
.ci-trigger{display:inline-flex;flex-shrink:0;align-items:center;gap:6px;white-space:nowrap;border:1px solid #f3d9a7;border-radius:9px;padding:7px 10px;font-size:12px;font-weight:600;color:#a56817;background:#fff8e9}.ci-trigger.checked{color:#079364;border-color:#b3e8d2;background:#edfbf5}
.ci-overlay { position: fixed; inset: 0; z-index: 60; display: grid; place-items: center; padding: 24px; background: #10282185; backdrop-filter: blur(3px); }
.ci-dialog {
  --ci-paper: #fff;
  --ci-text: #152820;
  --ci-muted: #52665d;
  --ci-line: #dae4df;
  --ci-soft: #edf8f2;
  --ci-warm: #f8f3e8;
  --ci-green: #13604a;
  --ci-amber: #925c13;
  --ci-trial-border: #b6d8ca;
  color: var(--ci-text);
  background: var(--ci-paper);
  width: min(620px, 100%);
  max-height: calc(100dvh - 48px);
  border-radius: 20px;
  box-shadow: 0 24px 80px #12251e40;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.ci-dialog.expanded { width: min(840px, 100%); }
.ci-header { display: flex; align-items: center; gap: 14px; flex-shrink: 0; padding: 24px 26px 18px; }
.ci-header-icon { display: grid; place-items: center; width: 38px; height: 38px; border-radius: 10px; color: var(--ci-green); background: var(--ci-soft); }
.ci-header h2 { font-size: 24px; font-weight: 720; }
.ci-close { margin-left: auto; display: grid; place-items: center; width: 42px; height: 42px; border: 1px solid var(--ci-line); border-radius: 10px; color: var(--ci-muted); }
.ci-close:hover { background: var(--ci-soft); }
.ci-body { padding: 0 26px 18px; overflow-y: auto; overscroll-behavior: contain; }
.ci-callout { display: flex; align-items: center; gap: 15px; padding: 18px; border-radius: 14px; background: var(--ci-soft); }
.ci-callout.locked { background: var(--ci-warm); }
.ci-callout-icon { flex: none; display: grid; place-items: center; width: 42px; height: 42px; border-radius: 12px; background: var(--ci-paper); color: var(--ci-green); }
.ci-callout.locked .ci-callout-icon { color: var(--ci-amber); }
.ci-callout h3, .ci-trial h3 { font-size: 17px; font-weight: 700; }
.ci-callout p, .ci-trial p { margin: 5px 0 0; font-size: 14px; line-height: 1.5; text-wrap: pretty; }
.ci-callout p.ci-muted { color: var(--ci-muted); font-size: 12px; }
.ci-callout p.ci-today-reward { color: var(--ci-green); font-size: 22px; font-weight: 700; font-variant-numeric: tabular-nums; }
.ci-today-reward small, .ci-stats small { margin-inline-start: 5px; font-size: 12px; color: var(--ci-muted); font-weight: 500; }
.ci-stats { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); margin: 16px 0; padding: 14px 0; border: 1px solid var(--ci-line); border-radius: 14px; }
.ci-stats > div { padding-inline: 18px; }
.ci-stats > div + div { border-inline-start: 1px solid var(--ci-line); }
.ci-stats dt { color: var(--ci-muted); font-size: 13px; }
.ci-stats dd { margin: 4px 0 0; color: var(--ci-green); font-size: 24px; font-weight: 700; font-variant-numeric: tabular-nums; overflow-wrap: anywhere; }
.ci-recharge { margin-bottom: 16px; padding: 16px 18px; border: 1px solid var(--ci-trial-border); border-radius: 14px; background: var(--ci-soft); }
.ci-recharge-heading { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.ci-recharge-heading h3 { display: flex; align-items: center; gap: 8px; font-size: 17px; font-weight: 700; }
.ci-recharge-heading h3 svg { flex: none; color: var(--ci-green); }
.ci-recharge-badge { flex: none; padding: 4px 9px; border-radius: 999px; background: var(--ci-warm); color: var(--ci-amber); font-size: 12px; font-weight: 600; }
.ci-recharge-badge.met { background: var(--ci-paper); color: var(--ci-green); }
.ci-recharge-amounts { display: flex; justify-content: space-between; gap: 14px; margin: 12px 0 9px; }
.ci-recharge-amounts > div:last-child { text-align: end; }
.ci-recharge-amounts dt { color: var(--ci-muted); font-size: 12px; }
.ci-recharge-amounts dd { color: var(--ci-green); font-size: 21px; font-weight: 700; font-variant-numeric: tabular-nums; overflow-wrap: anywhere; }
.ci-recharge-meter { appearance: none; display: block; width: 100%; height: 10px; overflow: hidden; border: 0; border-radius: 999px; background: var(--ci-line); }
.ci-recharge-meter::-webkit-progress-bar { border-radius: 999px; background: var(--ci-line); }
.ci-recharge-meter::-webkit-progress-value { border-radius: 999px; background: linear-gradient(90deg, #2eaa79, #147554); }
.ci-recharge-meter::-moz-progress-bar { border-radius: 999px; background: linear-gradient(90deg, #2eaa79, #147554); }
.ci-recharge-gap { margin-top: 10px; color: var(--ci-green); font-size: 14px; }
.ci-recharge-gap strong { margin-inline-start: 3px; font-size: 21px; font-variant-numeric: tabular-nums; }
.ci-recharge-note { margin-top: 8px; color: var(--ci-muted); font-size: 12px; }
.ci-action { display: block; width: 100%; min-height: 44px; padding: 10px; border-radius: 10px; background: #166750; color: #fff; font-size: 15px; font-weight: 650; text-align: center; }
.ci-action:disabled { background: var(--ci-line); color: var(--ci-muted); cursor: default; }
.ci-note { margin: 10px 0 16px; color: var(--ci-muted); font-size: 12px; text-align: center; }
.ci-trial { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; gap: 12px; align-items: center; padding: 12px 16px; border: 1px solid var(--ci-trial-border); border-radius: 14px; background: var(--ci-soft); }
.ci-trial-icon { color: var(--ci-green); }
.ci-trial p { color: var(--ci-muted); }
.ci-trial-qr { display: block; padding: 4px; border-radius: 9px; background: #fff; }
.ci-trial-qr img { display: block; width: 140px; height: auto; }
.ci-footer { display: flex; justify-content: center; padding: 10px 0 0; }
.ci-footer .ci-link { color: var(--ci-green); font-size: 12px; }
.ci-columns{display:grid;grid-template-columns:minmax(0,1.5fr) minmax(0,1fr);gap:14px;align-items:start;margin-top:12px}.ci-calendar,.ci-stage{border:1px solid var(--ci-line);border-radius:14px;padding:13px}.ci-calendar-heading,.ci-stage-heading,.ci-records>div:first-child{display:flex;align-items:center;justify-content:space-between;gap:8px}.ci-columns h3{font-size:13px;font-weight:650}.ci-calendar-heading h3{border-left:3px solid #edae3d;padding-left:7px}.ci-month-nav{display:flex;align-items:center;gap:7px}.ci-month-nav button{border:1px solid var(--ci-line);border-radius:8px;display:grid;place-items:center;width:27px;height:27px}.ci-month-nav strong{font-size:11px;white-space:nowrap}.ci-hint{font-size:11px;color:var(--ci-muted);margin:10px 0 12px;padding-bottom:10px;border-bottom:1px solid var(--ci-line);line-height:1.6}
.ci-grid{display:grid;grid-template-columns:repeat(7,minmax(0,1fr));gap:6px}.ci-weekday{font-size:11px;color:var(--ci-muted);text-align:center;padding-bottom:6px}.ci-date{position:relative;display:grid;grid-template-columns:1fr auto;grid-template-rows:16px 17px;gap:7px 0;min-width:0;min-height:57px;padding:7px 4px;border:1px solid var(--ci-line);border-radius:9px;text-align:left;isolation:isolate;font-size:12px}.ci-date-check{color:#09ad76;justify-self:end}.ci-date.signed{background:var(--ci-soft);border-color:#a9e8ce}.ci-date.node{background:var(--ci-warm);border-color:#f0d09b}.ci-date.node.signed{background:var(--ci-soft)}.ci-date.today{border-color:#dcab53;box-shadow:inset 0 0 0 1px #f7e7bf}.ci-date.selected:not(.today){outline:1px solid #0aab79;outline-offset:1px}.ci-node-label{font-size:10px;white-space:nowrap;color:#a16a1a;align-self:start}.ci-date-reward{grid-column:1/-1;font-size:11px;font-weight:650;color:#05996a;white-space:nowrap;align-self:end;width:max-content;max-width:100%;font-variant-numeric:tabular-nums}.ci-date.node:not(.signed) .ci-date-reward{color:#ac7626}.ci-stamp{position:absolute;right:0;bottom:10px;transform:rotate(-24deg);font-size:11px;color:#0aa877;opacity:.13;z-index:-1;pointer-events:none}
.ci-legend{display:flex;gap:11px;flex-wrap:wrap;font-size:10px;color:var(--ci-muted);margin:12px 0 9px}.ci-legend span{display:flex;align-items:center;gap:5px}.ci-legend i{width:6px;height:6px;border-radius:50%;border:1px solid #0aab79}.ci-legend .today{border-color:#dba13b}.ci-legend .node{background:#ebb345;border-color:#ebb345}.ci-selection{font-size:11px;color:var(--ci-muted);line-height:1.6;overflow-wrap:anywhere}.ci-stage-heading{flex-wrap:wrap}.ci-stage-heading h3{display:flex;align-items:center;gap:5px}.ci-stage-heading svg{color:#db9d2c}.ci-stage-heading>span{font-size:11px;color:#a36b1e;padding:4px 7px;background:var(--ci-warm);border-radius:20px}.ci-muted{font-size:11px;color:var(--ci-muted);line-height:1.6;margin:9px 0 11px}
.ci-milestones{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px}.ci-milestone{padding:10px;border:1px solid var(--ci-line);border-radius:10px;display:flex;flex-direction:column;gap:7px;text-align:left;background:#f7faf8}.ci-milestone.complete{background:var(--ci-soft);border-color:#b2e9d3}.ci-milestone.next{background:var(--ci-warm);border-color:#f0d09b}.ci-milestone>span{display:flex;align-items:center;justify-content:space-between;width:100%;font-size:11px}.ci-milestone svg{color:#dca337}.ci-milestone strong{font-size:17px;font-weight:650;color:#05996a}.ci-milestone small{font-size:11px;color:var(--ci-muted)}
.ci-records{margin-top:14px;border-top:1px solid var(--ci-line);padding-top:11px}.ci-records h3{font-size:12px}.ci-link{display:inline-flex;align-items:center;gap:4px;font-size:11px;color:#a76d1f;padding:3px 0}.ci-record{display:flex;justify-content:space-between;align-items:center;gap:8px;margin-top:10px;font-size:11px;color:var(--ci-muted)}.ci-record small{display:block;font-size:10px;margin-top:3px}.ci-record strong{color:#05996a;white-space:nowrap;font-variant-numeric:tabular-nums}.ci-rules{margin-top:9px;color:var(--ci-muted);font-size:12px;line-height:1.8}.ci-rules p+p{margin-top:5px}.ci-empty{padding:30px;text-align:center;color:var(--ci-muted);font-size:13px}.ci-empty .ci-action{max-width:160px;margin:15px auto 0}.ci-dialog :is(button,a):focus-visible,.ci-trigger:focus-visible{outline:2px solid #0aa877;outline-offset:3px}.ci-month-nav button:disabled,.ci-milestone:disabled{opacity:.55}
.dark .ci-dialog{--ci-paper:#1c2721;--ci-text:#e8f1eb;--ci-muted:#b0c2b8;--ci-line:#36483d;--ci-soft:#193b2c;--ci-warm:#3a3020;--ci-green:#7fe0b3;--ci-amber:#efc071;--ci-trial-border:#396b55}.dark .ci-trigger{background:#3a3020;border-color:#71552a;color:#efc071}.dark .ci-trigger.checked{background:#173b2d;border-color:#31664c;color:#7fe0b3}.dark .ci-milestone{background:#25342b}.dark .ci-milestone.complete{background:var(--ci-soft);border-color:#30634b}.dark .ci-date.signed{border-color:#30634b}.dark .ci-date.node,.dark .ci-milestone.next{border-color:#795b2a}.dark .ci-date-reward,.dark .ci-milestone strong,.dark .ci-record strong{color:#63dca6}.dark .ci-node-label{color:#efc071}
@media (max-width: 640px) {
  .ci-overlay { padding: 12px; }
  .ci-dialog { max-height: calc(100dvh - 24px); border-radius: 16px; }
  .ci-header { padding: 14px 16px; }
  .ci-header h2 { font-size: 20px; }
  .ci-body { padding: 0 16px 12px; }
  .ci-callout { align-items: flex-start; gap: 10px; padding: 12px; }
  .ci-callout-icon { width: 32px; height: 32px; }
  .ci-callout p { font-size: 13px; }
  .ci-recharge { padding: 13px 12px; }
  .ci-recharge-heading h3 { font-size: 16px; }
  .ci-trial { grid-template-columns: minmax(0, 1fr) auto; padding: 10px 12px; }
  .ci-trial-icon { display: none; }
  .ci-trial-qr img { width: 144px; }
  .ci-columns { grid-template-columns: minmax(0, 1fr); }
  .ci-milestones { grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 6px; }
  .ci-milestone { padding: 8px 5px; }
  .ci-milestone svg { display: none; }
  .ci-milestone strong { font-size: 15px; }
  .ci-milestone small { font-size: 10px; }
  .ci-calendar { padding: 12px 9px; }
  .ci-grid { gap: 4px; }
  .ci-date { padding: 7px 3px; }
  .ci-month-nav { gap: 5px; }
}
@media (max-width: 350px) {
  .ci-trial { grid-template-columns: minmax(0, 1fr); justify-items: center; text-align: center; }
}
</style>
