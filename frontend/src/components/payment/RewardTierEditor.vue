<template>
  <div class="mt-5 overflow-x-auto">
    <table class="w-full min-w-[420px] text-left text-sm">
      <thead class="text-xs text-gray-500 dark:text-gray-400">
        <tr>
          <th class="pb-2">{{ t('payment.activities.threshold') }}</th>
          <th class="pb-2">{{ t('payment.activities.bonus') }}</th>
          <th class="pb-2"></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(tier, index) in modelValue" :key="index">
          <td class="pb-2 pr-3"><input :value="tier.threshold" type="number" min="0.01" step="0.01" class="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900" @input="updateTier(index, 'threshold', $event)" /></td>
          <td class="pb-2 pr-3"><input :value="tier.bonus" type="number" min="0.01" step="0.01" class="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-900" @input="updateTier(index, 'bonus', $event)" /></td>
          <td class="pb-2"><button type="button" class="rounded-md p-2 text-gray-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20" :title="t('common.delete')" @click="removeTier(index)"><Icon name="trash" size="sm" /></button></td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { RewardTier } from '@/types/payment'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ modelValue: RewardTier[] }>()
const emit = defineEmits<{ 'update:modelValue': [tiers: RewardTier[]] }>()
const { t } = useI18n()

function updateTier(index: number, field: keyof RewardTier, event: Event) {
  const value = Number((event.target as HTMLInputElement).value)
  emit('update:modelValue', props.modelValue.map((tier, tierIndex) => tierIndex === index ? { ...tier, [field]: value } : tier))
}

function removeTier(index: number) {
  emit('update:modelValue', props.modelValue.filter((_, tierIndex) => tierIndex !== index))
}
</script>
