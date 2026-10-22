<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex justify-center py-16" role="status">
        <span class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent" />
        <span class="sr-only">{{ t('inviteRewards.loading') }}</span>
      </div>
      <div v-else-if="!detail" class="card space-y-4 p-8 text-center">
        <p>{{ t('inviteRewards.loadFailed') }}</p>
        <button class="btn btn-primary" @click="load(1)">{{ t('inviteRewards.retry') }}</button>
      </div>
      <template v-else>
        <section class="card overflow-hidden border-primary-200 bg-gradient-to-br from-primary-50 via-white to-emerald-50 p-6 dark:border-primary-900/50 dark:from-primary-950 dark:via-dark-900 dark:to-dark-800 sm:p-8">
          <div class="flex flex-col gap-6 md:flex-row md:items-center md:justify-between">
            <div>
              <p class="text-sm font-medium text-primary-600 dark:text-primary-400">{{ t('inviteRewards.eyebrow') }}</p>
              <h2 class="mt-2 text-2xl font-bold text-gray-900 dark:text-white">{{ t('inviteRewards.title') }}</h2>
              <p v-if="detail.invite_bonus_enabled" class="mt-3 max-w-xl text-sm leading-6 text-gray-600 dark:text-dark-300">
                {{ t('inviteRewards.description', { amount: formatCurrency(detail.invite_bonus_amount) }) }}
              </p>
              <p v-else class="mt-3 text-sm text-amber-700 dark:text-amber-400">{{ t('inviteRewards.paused') }}</p>
            </div>
            <RouterLink to="/purchase" class="btn btn-secondary self-start md:self-auto">{{ t('inviteRewards.wallet') }}</RouterLink>
          </div>
        </section>

        <section class="grid gap-4 sm:grid-cols-2">
          <div class="card p-6">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('inviteRewards.invitedCount') }}</p>
            <p class="mt-3 text-3xl font-semibold tabular-nums text-gray-900 dark:text-white" data-testid="invited-count">{{ detail.invited_count.toLocaleString() }}</p>
            <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">{{ t('inviteRewards.countHint') }}</p>
          </div>
          <div class="card p-6">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('inviteRewards.totalBonus') }}</p>
            <p class="mt-3 text-3xl font-semibold tabular-nums text-emerald-600 dark:text-emerald-400" data-testid="total-bonus">{{ formatCurrency(detail.total_bonus) }}</p>
            <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">{{ t('inviteRewards.totalHint') }}</p>
          </div>
        </section>

        <section class="card p-6">
          <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('inviteRewards.shareTitle') }}</h3>
          <div class="mt-5 grid gap-5 lg:grid-cols-[1fr_2fr]">
            <div>
              <p class="mb-2 text-sm text-gray-500 dark:text-dark-400">{{ t('inviteRewards.code') }}</p>
              <div class="flex flex-wrap items-center gap-3 rounded-xl bg-gray-50 p-3 dark:bg-dark-800">
                <code class="min-w-0 flex-1 break-all font-semibold">{{ detail.aff_code }}</code>
                <button class="btn btn-secondary btn-sm" :disabled="!detail.invite_bonus_enabled" @click="copyToClipboard(detail.aff_code, t('inviteRewards.copied'))">{{ t('inviteRewards.copy') }}</button>
              </div>
            </div>
            <div>
              <p class="mb-2 text-sm text-gray-500 dark:text-dark-400">{{ t('inviteRewards.link') }}</p>
              <div class="flex flex-wrap items-center gap-3 rounded-xl bg-gray-50 p-3 dark:bg-dark-800">
                <code class="min-w-0 flex-1 break-all text-sm">{{ inviteLink }}</code>
                <button class="btn btn-primary btn-sm" :disabled="!detail.invite_bonus_enabled" @click="copyToClipboard(inviteLink, t('inviteRewards.copied'))">{{ t('inviteRewards.copyLink') }}</button>
              </div>
            </div>
          </div>
          <div class="mt-5 rounded-xl bg-primary-50 p-4 text-sm leading-6 text-primary-800 dark:bg-primary-900/20 dark:text-primary-200">
            <p v-if="detail.signup_bonus_enabled">{{ t('inviteRewards.signupRule', { amount: formatCurrency(detail.signup_bonus_amount) }) }}</p>
            <p v-if="detail.invite_bonus_enabled">{{ t('inviteRewards.inviteRule', { amount: formatCurrency(detail.invite_bonus_amount) }) }}</p>
            <p>{{ t('inviteRewards.rules') }}</p>
          </div>
        </section>

        <section class="card p-6">
          <div class="flex items-center justify-between gap-3">
            <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('inviteRewards.records') }}</h3>
            <button class="btn btn-secondary btn-sm" :disabled="refreshing" @click="load(page)">{{ t('inviteRewards.refresh') }}</button>
          </div>
          <p v-if="detail.records.length === 0" class="mt-5 rounded-xl border border-dashed border-gray-200 px-4 py-10 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">{{ t(detail.invite_bonus_enabled ? 'inviteRewards.empty' : 'inviteRewards.emptyPaused') }}</p>
          <div v-else class="mt-4 overflow-x-auto">
            <table class="w-full min-w-[560px] text-left text-sm">
              <thead><tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                <th class="px-3 py-3 font-medium">{{ t('inviteRewards.account') }}</th>
                <th class="px-3 py-3 font-medium">{{ t('inviteRewards.date') }}</th>
                <th class="px-3 py-3 font-medium">{{ t('inviteRewards.amount') }}</th>
                <th class="px-3 py-3 font-medium">{{ t('inviteRewards.status') }}</th>
              </tr></thead>
              <tbody><tr v-for="record in detail.records" :key="record.user_id" class="border-b border-gray-100 last:border-0 dark:border-dark-800">
                <td class="px-3 py-4">{{ record.email }}</td>
                <td class="px-3 py-4 text-gray-500 dark:text-dark-400">{{ formatDateTime(record.created_at) }}</td>
                <td class="px-3 py-4 font-medium tabular-nums text-emerald-600 dark:text-emerald-400">{{ formatCurrency(record.amount) }}</td>
                <td class="px-3 py-4"><span class="rounded-full bg-gray-100 px-2.5 py-1 text-xs dark:bg-dark-700">{{ t(record.status === 'credited' ? 'inviteRewards.credited' : 'inviteRewards.noReward') }}</span></td>
              </tr></tbody>
            </table>
          </div>
          <div v-if="detail.invited_count > detail.page_size" class="mt-5 flex items-center justify-end gap-3 text-sm">
            <button class="btn btn-secondary btn-sm" :disabled="refreshing || page <= 1" @click="load(page - 1)">{{ t('inviteRewards.previous') }}</button>
            <span>{{ page }} / {{ Math.ceil(detail.invited_count / detail.page_size) }}</span>
            <button class="btn btn-secondary btn-sm" :disabled="refreshing || page * detail.page_size >= detail.invited_count" @click="load(page + 1)">{{ t('inviteRewards.next') }}</button>
          </div>
        </section>

        <section v-if="detail.legacy_quota > 0 || detail.legacy_frozen_quota > 0" class="card p-6">
          <h3 class="font-semibold">{{ t('inviteRewards.legacyTitle') }}</h3>
          <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('inviteRewards.legacyHint', { available: formatCurrency(detail.legacy_quota), frozen: formatCurrency(detail.legacy_frozen_quota) }) }}</p>
          <button class="btn btn-secondary mt-4" :disabled="transferring || detail.legacy_quota <= 0" @click="transferLegacy">{{ t('inviteRewards.legacyTransfer') }}</button>
        </section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import userAPI from '@/api/user'
import type { SignupPromotionDetail } from '@/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()
const detail = ref<SignupPromotionDetail | null>(null)
const loading = ref(true)
const refreshing = ref(false)
const transferring = ref(false)
const page = ref(1)
const inviteLink = computed(() => detail.value ? `${window.location.origin}/register?aff=${encodeURIComponent(detail.value.aff_code)}` : '')

async function load(targetPage: number) {
  if (refreshing.value) return
  refreshing.value = true
  try {
    detail.value = await userAPI.getSignupPromotion(targetPage)
    page.value = detail.value.page
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('inviteRewards.loadFailed')))
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function transferLegacy() {
  if (transferring.value) return
  transferring.value = true
  try {
    await userAPI.transferAffiliateQuota()
    appStore.showSuccess(t('inviteRewards.legacySuccess'))
    await Promise.all([load(page.value), authStore.refreshUser()])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('inviteRewards.legacyFailed')))
  } finally {
    transferring.value = false
  }
}

onMounted(() => { void load(1) })
</script>
