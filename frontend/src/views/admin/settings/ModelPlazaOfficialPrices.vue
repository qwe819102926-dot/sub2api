<template>
  <fieldset
    :disabled="disabled"
    aria-labelledby="model-plaza-official-prices"
    class="space-y-3 border-t border-gray-100 pt-5 dark:border-dark-700"
  >
    <div class="flex items-start justify-between gap-3">
      <div>
        <h3 id="model-plaza-official-prices" class="text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.settings.features.modelPlaza.officialPrices') }}
        </h3>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.features.modelPlaza.officialPricesHint') }}
        </p>
      </div>
      <button type="button" class="btn btn-secondary btn-sm shrink-0" @click="addRow">
        {{ t('admin.settings.features.modelPlaza.addOfficialPrice') }}
      </button>
    </div>
    <div
      v-for="(row, index) in rows"
      :key="index"
      class="space-y-3 rounded-lg border border-gray-200 p-3 dark:border-dark-700"
    >
      <div class="flex items-end justify-between gap-3">
        <label class="block min-w-0 flex-1 sm:max-w-sm">
          <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">
            {{ t('modelPlaza.table.model') }}
          </span>
          <input
            :value="row.model"
            type="text"
            class="input font-mono text-sm"
            placeholder="deepseek-v4-pro"
            spellcheck="false"
            @input="updateRow(index, { model: ($event.target as HTMLInputElement).value })"
          />
        </label>
        <button
          type="button"
          class="btn btn-secondary btn-sm shrink-0"
          @click="removeRow(index)"
        >
          {{ t('admin.settings.features.modelPlaza.removeOfficialPrice') }}
        </button>
      </div>
      <div class="grid grid-cols-2 gap-3 lg:grid-cols-5">
        <label v-for="field in priceFields" :key="field.key" class="block min-w-0">
          <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">
            {{ t(field.label) }}
          </span>
          <input
            :value="row[field.key] ?? ''"
            type="number"
            min="0"
            step="any"
            inputmode="decimal"
            class="input font-mono text-sm"
            :placeholder="t('admin.settings.features.modelPlaza.defaultOfficialPrice')"
            @input="updatePrice(index, field.key, $event)"
          />
        </label>
      </div>
    </div>
  </fieldset>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ModelPlazaOfficialPriceOverride } from '@/api/admin/settings'

defineProps<{ disabled: boolean }>()
const rows = defineModel<ModelPlazaOfficialPriceOverride[]>({ required: true })
const { t } = useI18n()

type PriceField = Exclude<keyof ModelPlazaOfficialPriceOverride, 'model'>
const priceFields = [
  { key: 'input_price', label: 'modelPlaza.table.input' },
  { key: 'output_price', label: 'modelPlaza.table.output' },
  { key: 'cache_read_price', label: 'admin.settings.features.modelPlaza.cacheRead' },
  { key: 'cache_write_price', label: 'admin.settings.features.modelPlaza.cacheWrite' },
  { key: 'cache_write_1h_price', label: 'admin.settings.features.modelPlaza.cacheWrite1h' },
] as const

function addRow() {
  rows.value = [...rows.value, {
    model: '',
    input_price: null,
    output_price: null,
    cache_read_price: null,
    cache_write_price: null,
    cache_write_1h_price: null,
  }]
}

function removeRow(index: number) {
  rows.value = rows.value.filter((_, rowIndex) => rowIndex !== index)
}

function updateRow(index: number, patch: Partial<ModelPlazaOfficialPriceOverride>) {
  rows.value = rows.value.map((row, rowIndex) => rowIndex === index ? { ...row, ...patch } : row)
}

function updatePrice(index: number, field: PriceField, event: Event) {
  const input = event.target as HTMLInputElement
  const price = input.value || input.validity.badInput ? input.valueAsNumber : null
  updateRow(index, { [field]: price })
}
</script>
