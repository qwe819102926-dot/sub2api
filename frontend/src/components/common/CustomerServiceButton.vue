<template>
  <button
    @click="openModal"
    class="relative flex h-10 items-center justify-center gap-1.5 rounded-xl border border-emerald-300 bg-white px-3 text-gray-700 shadow-sm transition-all hover:border-emerald-400 hover:bg-emerald-50 hover:text-emerald-700 hover:shadow dark:border-emerald-700 dark:bg-dark-800 dark:text-gray-200 dark:hover:border-emerald-500 dark:hover:bg-emerald-900/20 dark:hover:text-emerald-300"
    :aria-label="buttonLabel"
    :title="buttonLabel"
  >
    <Icon name="headphones" size="sm" />
    <span class="text-sm font-medium">{{ buttonLabel }}</span>
  </button>

  <Teleport to="body">
    <Transition name="modal-fade">
      <div
        v-if="isModalOpen"
        class="fixed inset-0 z-[100] flex items-start justify-center overflow-y-auto bg-gradient-to-br from-black/70 via-black/60 to-black/70 p-4 pt-[8vh] backdrop-blur-md"
        @click="closeModal"
      >
        <div
          class="w-full max-w-[560px] overflow-hidden rounded-2xl bg-white shadow-2xl ring-1 ring-black/5 dark:bg-dark-800 dark:ring-white/10"
          @click.stop
        >
          <!-- Header -->
          <div class="flex items-center gap-3 border-b border-gray-100 px-6 py-5 dark:border-dark-700">
            <div class="flex h-11 w-11 items-center justify-center rounded-xl bg-emerald-50 text-emerald-600 dark:bg-emerald-900/20 dark:text-emerald-400">
              <Icon name="headphones" size="lg" />
            </div>
            <div class="min-w-0">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ panelTitle }}
              </h2>
              <p v-if="panelSubtitle" class="truncate text-xs text-gray-500 dark:text-dark-400">
                {{ panelSubtitle }}
              </p>
            </div>
            <button
              @click="closeModal"
              class="ml-auto flex h-9 w-9 items-center justify-center rounded-lg text-gray-500 transition-all hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-gray-300"
              :aria-label="t('common.close')"
            >
              <Icon name="x" size="sm" />
            </button>
          </div>

          <!-- Body -->
          <div class="max-h-[65vh] overflow-y-auto px-6 py-5">
            <div class="grid grid-cols-1 gap-4">
              <div class="rounded-xl border border-gray-100 bg-gray-50/60 p-4 dark:border-dark-700 dark:bg-dark-800">
                <div class="mb-2 flex items-center justify-between">
                  <span class="text-sm font-medium text-gray-600 dark:text-dark-400">
                    {{ t('common.customerService.qqLabel') }}
                  </span>
                  <button
                    @click="copyValue(qqNumber)"
                    class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-emerald-600 transition-colors hover:bg-emerald-50 dark:text-emerald-400 dark:hover:bg-emerald-900/20"
                  >
                    <Icon name="copy" size="xs" />
                    {{ copiedValue === qqNumber ? t('common.customerService.copied') : t('common.customerService.copy') }}
                  </button>
                </div>

                <p class="text-xl font-bold tracking-wide text-gray-900 dark:text-white">{{ qqNumber }}</p>

                <div class="mt-3 flex justify-center">
                  <img
                    src="/customer-service-qq.jpg"
                    :alt="t('common.customerService.qqLabel')"
                    class="h-64 w-64 rounded-lg border border-gray-200 object-contain dark:border-dark-600 dark:bg-white/5"
                    loading="lazy"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const isModalOpen = ref(false)
const copiedValue = ref<string | null>(null)
let copyTimer: ReturnType<typeof setTimeout> | null = null

const qqNumber = '819102926'
const buttonLabel = computed(() => t('common.customerService.buttonLabel'))
const panelTitle = computed(() => t('common.contactSupport'))
const panelSubtitle = computed(() => t('common.customerService.subtitle'))

function openModal() {
  isModalOpen.value = true
  copiedValue.value = null
}

function closeModal() {
  isModalOpen.value = false
}

async function copyValue(value: string) {
  try {
    await navigator.clipboard.writeText(value)
  } catch {
    // Fallback for older browsers / insecure contexts.
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

function handleEscape(e: KeyboardEvent) {
  if (e.key === 'Escape') closeModal()
}

onMounted(() => {
  document.addEventListener('keydown', handleEscape)
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleEscape)
  if (copyTimer) clearTimeout(copyTimer)
  document.body.style.overflow = ''
})


watch(isModalOpen, (open) => {
  document.body.style.overflow = open ? 'hidden' : ''
})
</script>

<style scoped>
.modal-fade-enter-active {
  transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}
.modal-fade-leave-active {
  transition: all 0.2s cubic-bezier(0.4, 0, 1, 1);
}
.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}
.modal-fade-enter-from > div {
  transform: scale(0.94) translateY(-12px);
  opacity: 0;
}
.modal-fade-leave-to > div {
  transform: scale(0.96) translateY(-8px);
  opacity: 0;
}
.overflow-y-auto::-webkit-scrollbar {
  width: 8px;
}
.overflow-y-auto::-webkit-scrollbar-track {
  background: transparent;
}
.overflow-y-auto::-webkit-scrollbar-thumb {
  background: linear-gradient(to bottom, #cbd5e1, #94a3b8);
  border-radius: 4px;
}
.dark .overflow-y-auto::-webkit-scrollbar-thumb {
  background: linear-gradient(to bottom, #4b5563, #374151);
}
</style>
