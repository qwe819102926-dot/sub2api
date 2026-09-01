<template>
  <div ref="panelRef" class="relative">
    <button
      type="button"
      @click="togglePanel"
      class="relative flex h-10 items-center justify-center gap-1.5 rounded-xl border border-emerald-300 bg-white px-3 text-gray-700 shadow-sm transition-all hover:border-emerald-400 hover:bg-emerald-50 hover:text-emerald-700 hover:shadow dark:border-emerald-700 dark:bg-dark-800 dark:text-gray-200 dark:hover:border-emerald-500 dark:hover:bg-emerald-900/20 dark:hover:text-emerald-300"
      :aria-expanded="isPanelOpen"
      :aria-label="buttonLabel"
      :title="buttonLabel"
    >
      <Icon name="headphones" size="sm" />
      <span class="text-sm font-medium">{{ buttonLabel }}</span>
    </button>

    <Transition name="support-panel">
      <div
        v-if="isPanelOpen"
        class="absolute right-0 top-full z-[100] mt-2 w-[min(20rem,calc(100vw-2rem))] overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xl ring-1 ring-black/5 dark:border-dark-700 dark:bg-dark-800 dark:ring-white/10"
        @click.stop
      >
        <div class="flex items-center gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700">
          <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-emerald-50 text-emerald-600 dark:bg-emerald-900/20 dark:text-emerald-400">
            <Icon name="headphones" size="md" />
          </div>
          <div class="min-w-0">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ panelTitle }}</h2>
            <p v-if="panelSubtitle" class="truncate text-xs text-gray-500 dark:text-dark-400">{{ panelSubtitle }}</p>
          </div>
          <button
            type="button"
            @click="closePanel"
            class="ml-auto flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-gray-200"
            :aria-label="t('common.close')"
          >
            <Icon name="x" size="xs" />
          </button>
        </div>

        <div class="p-3">
          <div class="rounded-lg border border-gray-100 bg-gray-50/70 p-3 dark:border-dark-700 dark:bg-dark-800">
            <div class="mb-1 flex items-center justify-between">
              <span class="text-xs font-medium text-gray-500 dark:text-dark-400">
                {{ t('common.customerService.qqLabel') }}
              </span>
              <button
                type="button"
                @click="copyValue(qqNumber)"
                class="inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-xs font-medium text-emerald-600 transition-colors hover:bg-emerald-50 dark:text-emerald-400 dark:hover:bg-emerald-900/20"
              >
                <Icon name="copy" size="xs" />
                {{ copiedValue === qqNumber ? t('common.customerService.copied') : t('common.customerService.copy') }}
              </button>
            </div>
            <p class="text-lg font-bold tracking-wide text-gray-900 dark:text-white">{{ qqNumber }}</p>
            <div class="mt-2 flex justify-center rounded-lg bg-white p-2 dark:bg-dark-700">
              <img
                src="/customer-service-qq.jpg"
                :alt="t('common.customerService.qqLabel')"
                width="240"
                height="244"
                class="h-60 w-60 rounded-md object-contain"
                decoding="async"
                fetchpriority="high"
              />
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const isPanelOpen = ref(false)
const copiedValue = ref<string | null>(null)
const panelRef = ref<HTMLElement | null>(null)
let copyTimer: ReturnType<typeof setTimeout> | null = null

const qqNumber = '819102926'
const buttonLabel = computed(() => t('common.customerService.buttonLabel'))
const panelTitle = computed(() => t('common.contactSupport'))
const panelSubtitle = computed(() => t('common.customerService.subtitle'))

function togglePanel() {
  isPanelOpen.value = !isPanelOpen.value
  if (isPanelOpen.value) copiedValue.value = null
}

function closePanel() {
  isPanelOpen.value = false
}

async function copyValue(value: string) {
  try {
    await navigator.clipboard.writeText(value)
  } catch {
    const textarea = document.createElement('textarea')
    textarea.value = value
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
  }
  copiedValue.value = value
  if (copyTimer) clearTimeout(copyTimer)
  copyTimer = setTimeout(() => {
    copiedValue.value = null
  }, 1500)
}

function handleClickOutside(event: MouseEvent) {
  if (panelRef.value && !panelRef.value.contains(event.target as Node)) closePanel()
}

function handleEscape(event: KeyboardEvent) {
  if (event.key === 'Escape') closePanel()
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  document.addEventListener('keydown', handleEscape)

  // Warm the browser cache so the QR code is ready when the panel opens.
  const qrCode = new Image()
  qrCode.src = '/customer-service-qq.jpg'
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
  document.removeEventListener('keydown', handleEscape)
  if (copyTimer) clearTimeout(copyTimer)
})
</script>

<style scoped>
.support-panel-enter-active,
.support-panel-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
  transform-origin: top right;
}

.support-panel-enter-from,
.support-panel-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.98);
}
</style>
