<template>
  <BaseDialog :show="show" :title="t('admin.users.lotteryHistory')" width="wide" @close="$emit('close')">
    <div v-if="user" class="space-y-4">
      <div class="flex items-center justify-between gap-3 rounded-xl bg-gray-50 p-4 dark:bg-dark-700">
        <div class="min-w-0">
          <p class="truncate font-medium text-gray-900 dark:text-white">{{ user.email }}</p>
          <p class="text-xs text-gray-500 dark:text-gray-400">#{{ user.id }} <span v-if="user.username">· {{ user.username }}</span></p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="refreshKey++">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          {{ t('common.refresh') }}
        </button>
      </div>

      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3" data-test="lottery-summary">
        <div class="rounded-xl border border-emerald-200 bg-emerald-50 p-4 dark:border-emerald-900 dark:bg-emerald-950/30">
          <p class="text-xs text-emerald-700 dark:text-emerald-300">{{ t('admin.users.lotteryTotalPrize') }}</p>
          <p class="mt-1 text-xl font-semibold tabular-nums text-emerald-700 dark:text-emerald-300" data-test="lottery-total-prize">{{ history ? money(history.total_prize_amount) : '—' }}</p>
        </div>
        <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.users.lotteryTotalDraws') }}</p>
          <p class="mt-1 text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ history?.total ?? '—' }}</p>
        </div>
        <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.users.lotteryWinningDraws') }}</p>
          <p class="mt-1 text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ history?.winning_draws ?? '—' }}</p>
        </div>
      </div>
      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.users.lotteryHistoryHint') }}</p>

      <div v-if="loading" class="py-10 text-center text-sm text-gray-500" role="status">{{ t('common.loading') }}</div>
      <div v-else-if="failed" class="space-y-3 py-8 text-center" role="alert">
        <p class="text-sm text-red-600 dark:text-red-400">{{ t('admin.users.failedToLoadLotteryHistory') }}</p>
        <button type="button" class="btn btn-secondary" @click="refreshKey++">{{ t('common.refresh') }}</button>
      </div>
      <div v-else-if="!history?.items.length" class="py-10 text-center text-sm text-gray-500" data-test="lottery-empty">
        {{ t('admin.users.noLotteryHistory') }}
      </div>
      <div v-else class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-600">
        <table class="w-full whitespace-nowrap text-left text-sm">
          <thead class="bg-gray-50 text-gray-500 dark:bg-dark-700 dark:text-gray-400">
            <tr>
              <th class="px-4 py-3 font-medium">{{ t('admin.users.lotteryDrawTime') }}</th>
              <th class="px-4 py-3 font-medium">{{ t('admin.users.lotteryResult') }}</th>
              <th class="px-4 py-3 text-right font-medium">{{ t('admin.users.lotteryPrizeAmount') }}</th>
              <th class="px-4 py-3 font-medium">{{ t('admin.users.lotterySource') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-600">
            <tr v-for="draw in history.items" :key="draw.id" data-test="lottery-draw">
              <td class="px-4 py-3 text-gray-600 dark:text-gray-300">{{ formatDateTime(draw.created_at) }}</td>
              <td class="px-4 py-3">
                <span :class="draw.is_winner ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-500 dark:text-gray-400'">
                  {{ t(draw.is_winner ? 'admin.users.lotteryWon' : 'admin.users.lotteryNotWon') }}
                </span>
              </td>
              <td class="px-4 py-3 text-right font-semibold tabular-nums" :class="draw.is_winner ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-500'">{{ money(draw.prize_amount) }}</td>
              <td class="px-4 py-3 text-gray-500 dark:text-gray-400">
                {{ draw.order_id === null ? t('admin.users.lotteryAdminSource') : t('admin.users.lotteryOrderSource', { id: draw.order_id }) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <Pagination v-if="history && history.total > pageSize" :total="history.total" :page="page" :page-size="pageSize" :show-page-size-selector="false" @update:page="page = $event" />
    </div>
    <template #footer>
      <button type="button" class="btn btn-secondary" @click="$emit('close')">{{ t('common.close') }}</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { UserLotteryHistory } from '@/api/admin/users'
import type { AdminUser } from '@/types'
import { formatDateTime, formatPoints } from '@/utils/format'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
defineEmits<{ close: [] }>()
const { t } = useI18n()
const pageSize = 20
const page = ref(1)
const refreshKey = ref(0)
const loading = ref(false)
const failed = ref(false)
const history = ref<UserLotteryHistory | null>(null)
const money = formatPoints

watch(() => [props.show, props.user?.id], () => { page.value = 1 })
watch(() => [props.show, props.user?.id, page.value, refreshKey.value] as const, async ([show, userID, currentPage], _, onCleanup) => {
  history.value = null
  failed.value = false
  loading.value = false
  if (!show || !userID) return
  const controller = new AbortController()
  onCleanup(() => controller.abort())
  loading.value = true
  try {
    const data = await adminAPI.users.getLotteryHistory(userID, currentPage, pageSize, { signal: controller.signal })
    if (!controller.signal.aborted) history.value = data
  } catch {
    if (!controller.signal.aborted) failed.value = true
  } finally {
    if (!controller.signal.aborted) loading.value = false
  }
}, { immediate: true })
</script>
