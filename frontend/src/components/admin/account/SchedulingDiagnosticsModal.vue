<template>
  <BaseDialog :show="show" :title="t('schedulingDiagnostics.title')" width="extra-wide" @close="$emit('close')">
    <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ t('schedulingDiagnostics.description') }}</p>
    <div class="mb-4 flex flex-wrap gap-3">
      <select v-model="selectedGroup" class="input" :aria-label="t('schedulingDiagnostics.group')" @change="load">
        <option value="">{{ t('schedulingDiagnostics.allGroups') }}</option>
        <option v-for="group in groups" :key="group.id" :value="String(group.id)">{{ group.name }}</option>
      </select>
      <button class="btn btn-secondary" :disabled="loading" @click="load">{{ t('schedulingDiagnostics.refresh') }}</button>
    </div>
    <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
    <p v-else-if="loading" role="status" class="text-sm text-gray-500">{{ t('schedulingDiagnostics.loading') }}</p>
    <p v-else-if="!records.length" class="rounded-lg bg-gray-50 p-5 text-sm text-gray-500 dark:bg-dark-800">{{ t('schedulingDiagnostics.empty') }}</p>
    <template v-else>
      <select v-model="selectedID" class="input mb-4 w-full" :aria-label="t('schedulingDiagnostics.request')">
        <option v-for="record in records" :key="record.id" :value="record.id">
          {{ new Date(record.at).toLocaleTimeString() }} · {{ groupName(record.group_id) }} · {{ record.model }} → #{{ record.selected_id || '—' }}
        </option>
      </select>
      <template v-if="current">
        <div class="mb-3 flex flex-wrap gap-x-5 gap-y-2 text-sm">
          <span>{{ t('schedulingDiagnostics.selected') }}: <strong>{{ current.selected_id ? `#${current.selected_id}` : t('schedulingDiagnostics.none') }}</strong></span>
          <span>{{ t('schedulingDiagnostics.layer') }}: {{ translated('layers', current.layer) }}</span>
          <span>{{ t('schedulingDiagnostics.topK') }}: {{ current.top_k }}</span>
        </div>
        <p v-if="!current.cost_enabled" class="mb-2 text-sm text-amber-600">{{ t('schedulingDiagnostics.costOff') }}</p>
        <p v-if="!current.sticky_weighted" class="mb-2 text-sm text-amber-600">{{ t('schedulingDiagnostics.stickyOff') }}</p>
        <p v-if="!current.accounts.length" class="text-sm text-gray-500">{{ t(current.selected_id ? 'schedulingDiagnostics.direct' : 'schedulingDiagnostics.noCandidates') }}</p>
        <p v-if="current.truncated" class="mb-2 text-sm text-amber-600">{{ t('schedulingDiagnostics.truncated') }}</p>
        <div v-if="current.accounts.length" class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
          <table class="w-full text-left text-sm">
            <thead class="bg-gray-50 dark:bg-dark-800"><tr>
              <th v-for="key in ['account','status','rate','latency','score','parts','order']" :key="key" class="whitespace-nowrap p-3">{{ t(`schedulingDiagnostics.${key}`) }}</th>
            </tr></thead>
            <tbody>
              <tr v-for="row in current.accounts" :key="row.id" :class="row.id === current.selected_id ? 'bg-emerald-50 dark:bg-emerald-900/20' : ''" class="border-t border-gray-100 dark:border-dark-700">
                <td class="p-3"><div class="min-w-28 font-medium">{{ row.name }}</div><div class="text-xs text-gray-500">#{{ row.id }}</div></td>
                <td class="p-3"><span v-if="row.id === current.selected_id" class="font-medium text-emerald-600">{{ t('schedulingDiagnostics.selected') }}</span><span v-else>{{ row.reason ? translated('reasons', row.reason) : t(row.score == null ? 'schedulingDiagnostics.notScored' : 'schedulingDiagnostics.scored') }}</span></td>
                <td class="whitespace-nowrap p-3">{{ row.rate == null ? '—' : `${row.rate}×` }}</td>
                <td class="p-3"><template v-if="row.ttft_ms != null"><div>{{ Math.round(row.ttft_ms) }} ms</div><div class="mt-1 text-xs text-gray-500">{{ t('schedulingDiagnostics.freshness') }} {{ Math.round(row.latency_freshness * 100) }}%</div></template><span v-else>—</span><div v-if="row.score != null" class="mt-1 text-xs text-gray-500">{{ t('schedulingDiagnostics.errorRate') }} {{ (row.error_rate * 100).toFixed(1) }}%</div></td>
                <td class="p-3 font-semibold tabular-nums">{{ row.score == null ? '—' : row.score.toFixed(3) }}</td>
                <td class="p-3"><div v-if="row.parts" class="flex min-w-64 flex-wrap gap-x-3 gap-y-1 text-xs"><span v-for="(value, key) in row.parts" :key="key">{{ t(`schedulingDiagnostics.partsLabels.${key}`) }} {{ value.toFixed(3) }}</span></div></td>
                <td class="p-3">{{ row.order || '—' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p class="mt-3 text-xs text-gray-500">{{ t('schedulingDiagnostics.orderHint') }}</p>
      </template>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { getSchedulingDiagnostics, type SchedulingDiagnostic } from '@/api/admin/schedulingDiagnostics'

const props = defineProps<{ show: boolean; group?: string; groups: { id: number; name: string }[] }>()
defineEmits<{ close: [] }>()
const { t, te } = useI18n()
const selectedGroup = ref('')
const records = ref<SchedulingDiagnostic[]>([])
const selectedID = ref<number>()
const loading = ref(false)
const error = ref('')
let generation = 0
const current = computed(() => records.value.find(record => record.id === selectedID.value))
const groupName = (id: number) => props.groups.find(group => group.id === id)?.name ?? `#${id}`
function translated(kind: string, code: string) {
  const key = `schedulingDiagnostics.${kind}.${code}`
  return te(key) ? t(key) : code || '—'
}
async function load() {
  const request = ++generation
  loading.value = true
  error.value = ''
  try {
    const data = await getSchedulingDiagnostics(Number(selectedGroup.value) || undefined)
    if (request !== generation) return
    records.value = data
    if (!data.some(record => record.id === selectedID.value)) selectedID.value = data[0]?.id
  } catch {
    if (request === generation) error.value = t('schedulingDiagnostics.loadError')
  } finally {
    if (request === generation) loading.value = false
  }
}
watch(() => props.show, show => {
  if (show) {
    selectedGroup.value = props.group && Number(props.group) > 0 ? props.group : ''
    void load()
  } else {
    generation++
  }
}, { immediate: true })
</script>
