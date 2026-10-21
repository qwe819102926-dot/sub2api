<template>
  <BaseDialog :show="show" :title="t('admin.users.adjustBonusBalanceTitle')" width="narrow" @close="$emit('close')">
    <form v-if="user" id="bonus-balance-form" class="space-y-5" @submit.prevent="handleSubmit">
      <div class="flex items-center gap-3 rounded-xl bg-gray-50 p-4 dark:bg-dark-700">
        <div class="flex h-10 w-10 items-center justify-center rounded-full bg-primary-100">
          <span class="text-lg font-medium text-primary-700">{{ user.email.charAt(0).toUpperCase() }}</span>
        </div>
        <div class="min-w-0 flex-1">
          <p class="truncate font-medium text-gray-900 dark:text-gray-100">{{ user.email }}</p>
          <p class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.users.currentBonusBalance') }}:
            <span
              data-test="current-bonus-balance"
              :title="bonusBalanceKnown ? undefined : t('admin.users.bonusBalanceUnavailable')"
            >{{ bonusBalanceKnown ? `$${formatAmount(currentBonusBalance)}` : '—' }}</span>
          </p>
        </div>
      </div>

      <div class="grid grid-cols-2 gap-2">
        <button
          type="button"
          data-test="bonus-operation-add"
          class="rounded-lg border px-3 py-2 text-sm font-medium transition-colors"
          :class="operation === 'add'
            ? 'border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-300'
            : 'border-gray-200 text-gray-600 hover:bg-gray-50 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-700'"
          @click="operation = 'add'"
        >
          {{ t('admin.users.bonusBalanceAdd') }}
        </button>
        <button
          type="button"
          data-test="bonus-operation-subtract"
          class="rounded-lg border px-3 py-2 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50"
          :class="operation === 'subtract'
            ? 'border-amber-300 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300'
            : 'border-gray-200 text-gray-600 hover:bg-gray-50 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-700'"
          :disabled="!bonusBalanceKnown"
          @click="operation = 'subtract'"
        >
          {{ t('admin.users.bonusBalanceSubtract') }}
        </button>
      </div>

      <div>
        <label class="input-label">{{ t('admin.users.bonusBalanceAmount') }}</label>
        <div class="relative flex gap-2">
          <div class="relative flex-1">
            <div class="absolute left-3 top-1/2 -translate-y-1/2 font-medium text-gray-500">$</div>
            <input
              v-model.number="form.amount"
              data-test="bonus-amount-input"
              type="number"
              step="any"
              min="0"
              required
              class="input pl-8"
            />
          </div>
          <button
            v-if="operation === 'subtract'"
            type="button"
            class="btn btn-secondary whitespace-nowrap"
            @click="fillAllBonus"
          >
            {{ t('admin.users.withdrawAll') }}
          </button>
        </div>
      </div>

      <div>
        <label class="input-label">{{ t('admin.users.notes') }}</label>
        <textarea v-model="form.notes" rows="3" class="input"></textarea>
      </div>

      <div
        v-if="bonusBalanceKnown && form.amount > 0"
        data-test="bonus-preview"
        class="rounded-xl border p-4"
        :class="previewNegative
          ? 'border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-950'
          : 'border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-950'"
      >
        <div class="flex items-center justify-between text-sm">
          <span class="text-gray-700 dark:text-gray-300">{{ t('admin.users.newBonusBalance') }}:</span>
          <span class="font-bold" :class="previewNegative ? 'text-red-600 dark:text-red-400' : 'text-gray-900 dark:text-gray-100'">
            ${{ formatAmount(calculateNewBonusBalance()) }}
          </span>
        </div>
      </div>
    </form>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="$emit('close')">{{ t('common.cancel') }}</button>
        <button
          type="submit"
          form="bonus-balance-form"
          data-test="bonus-submit"
          class="btn btn-primary"
          :disabled="submitting || !form.amount || previewNegative"
        >
          {{ submitting ? t('common.saving') : t('admin.users.adjustBonusBalanceAction') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { AdminUser } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
const emit = defineEmits(['close', 'success'])
const { t } = useI18n()
const appStore = useAppStore()

const submitting = ref(false)
const operation = ref<'add' | 'subtract'>('add')
const form = reactive({ amount: 0, notes: '' })

const bonusBalanceKnown = computed(() => props.user?.bonus_balance != null && Number.isFinite(Number(props.user.bonus_balance)))
const currentBonusBalance = computed(() => bonusBalanceKnown.value ? Number(props.user?.bonus_balance) : 0)
const previewNegative = computed(() => bonusBalanceKnown.value && form.amount > 0 && calculateNewBonusBalance() < 0)

watch(() => props.show, (visible) => {
  if (visible) {
    operation.value = 'add'
    form.amount = 0
    form.notes = ''
  }
})

const formatAmount = (value: number) => (Number(value) || 0).toFixed(2)

const fillAllBonus = () => {
  form.amount = currentBonusBalance.value
}

const calculateNewBonusBalance = () => {
  if (!props.user) return 0
  const result = operation.value === 'add'
    ? currentBonusBalance.value + (Number(form.amount) || 0)
    : currentBonusBalance.value - (Number(form.amount) || 0)
  return Math.abs(result) < 1e-10 ? 0 : result
}

const handleSubmit = async () => {
  if (!props.user) return
  if (!form.amount || form.amount <= 0) {
    appStore.showError(t('admin.users.amountRequired'))
    return
  }
  if (operation.value === 'subtract' && !bonusBalanceKnown.value) {
    appStore.showError(t('admin.users.bonusBalanceUnavailable'))
    return
  }
  if (operation.value === 'subtract' && form.amount > currentBonusBalance.value) {
    appStore.showError(t('admin.users.insufficientBonusBalance'))
    return
  }
  submitting.value = true
  try {
    await adminAPI.users.updateBonusBalance(props.user.id, form.amount, operation.value, form.notes)
    appStore.showSuccess(t('admin.users.adjustBonusBalanceSuccess'))
    emit('success')
    emit('close')
  } catch (error: any) {
    console.error('Failed to update bonus balance:', error)
    appStore.showError(error.response?.data?.detail || t('admin.users.failedToAdjustBonusBalance'))
  } finally {
    submitting.value = false
  }
}
</script>
