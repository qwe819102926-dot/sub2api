<template>
  <BaseDialog :show="show" :title="t('admin.users.lotteryChancesTitle')" width="narrow" @close="$emit('close')">
    <form v-if="user" id="lottery-chances-form" class="space-y-5" @submit.prevent="submit">
      <div class="flex items-center gap-3 rounded-xl bg-gray-50 p-4 dark:bg-dark-700">
        <div class="flex h-10 w-10 items-center justify-center rounded-full bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
          <Icon name="trophy" size="md" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="truncate font-medium text-gray-900 dark:text-gray-100">{{ user.email }}</p>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.users.currentLotteryChances') }}: {{ current }}</p>
        </div>
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.lotteryChancesLabel') }}</label>
        <input v-model.number="remaining" type="number" min="0" max="1000000" step="1" required class="input" />
        <p class="input-hint">{{ t('admin.users.lotteryChancesHint') }}</p>
      </div>
    </form>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="$emit('close')">{{ t('common.cancel') }}</button>
        <button type="submit" form="lottery-chances-form" class="btn btn-primary" :disabled="loading || remaining < 0">
          {{ loading ? t('common.saving') : t('common.confirm') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { AdminUser } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
const emit = defineEmits(['close', 'success'])
const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const current = ref(0)
const remaining = ref(0)

watch(() => [props.show, props.user] as const, async ([show, user]) => {
  if (!show || !user) return
  loading.value = true
  try {
    const data = await adminAPI.users.getLotteryChances(user.id)
    current.value = data.remaining
    remaining.value = data.remaining
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.users.failedToLoadLotteryChances'))
  } finally {
    loading.value = false
  }
}, { immediate: true })

const submit = async () => {
  if (!props.user || !Number.isInteger(remaining.value) || remaining.value < 0 || remaining.value > 1000000) {
    appStore.showError(t('admin.users.lotteryChancesInvalid'))
    return
  }
  loading.value = true
  try {
    await adminAPI.users.updateLotteryChances(props.user.id, remaining.value)
    current.value = remaining.value
    appStore.showSuccess(t('admin.users.lotteryChancesUpdated'))
    emit('success')
    emit('close')
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.users.failedToUpdateLotteryChances'))
  } finally {
    loading.value = false
  }
}
</script>
