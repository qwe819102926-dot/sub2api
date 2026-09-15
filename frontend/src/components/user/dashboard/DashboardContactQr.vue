<template>
  <section
    class="overflow-hidden rounded-2xl border border-gray-100 bg-white/80 bg-[radial-gradient(circle,rgba(148,163,184,0.28)_1px,transparent_1.5px)] bg-[size:18px_18px] dark:border-dark-700/50 dark:bg-dark-800/40 dark:bg-[radial-gradient(circle,rgba(148,163,184,0.12)_1px,transparent_1.5px)]"
  >
    <div class="flex flex-col gap-6 p-5 lg:flex-row lg:items-center lg:justify-between lg:gap-8 lg:p-6">
      <div class="min-w-0 lg:max-w-sm">
        <h2 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-white sm:text-3xl">
          {{ t('dashboard.contactQr.hello') }}
        </h2>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-400 sm:text-base">
          {{ t('dashboard.contactQr.tagline') }}
        </p>
      </div>

      <div class="min-w-0 flex-1 rounded-2xl border border-primary-100/80 bg-white/90 p-4 shadow-sm dark:border-primary-900/40 dark:bg-dark-800/80 sm:p-5">
        <div class="flex flex-col gap-5 xl:flex-row xl:items-center xl:justify-between">
          <div class="min-w-0 xl:max-w-sm">
            <h3 class="text-lg font-semibold text-primary-700 dark:text-primary-300">
              {{ t('dashboard.contactQr.title') }}
            </h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
              {{ t('dashboard.contactQr.subtitle') }}
            </p>
            <div class="mt-3 inline-flex items-center gap-2 rounded-full bg-gradient-to-r from-primary-500 to-primary-600 px-3.5 py-1.5 text-xs font-medium text-white shadow-sm">
              <span>{{ t('dashboard.contactQr.cta') }}</span>
              <Icon name="arrowRight" size="xs" />
            </div>
            <p class="mt-3 text-xs font-medium leading-5 text-orange-500 dark:text-orange-400">
              {{ t('dashboard.contactQr.promo') }}
            </p>
          </div>

          <div class="flex flex-wrap items-start justify-center gap-4 sm:gap-6">
            <div v-for="item in items" :key="item.name" class="flex w-32 flex-col items-center text-center">
              <div class="rounded-xl border border-gray-100 bg-white p-1.5 shadow-sm dark:border-dark-700">
                <img
                  :src="item.image"
                  :alt="item.alt"
                  width="120"
                  height="120"
                  class="h-28 w-28 rounded-lg object-contain"
                  decoding="async"
                />
              </div>
              <p class="mt-2 flex items-center justify-center gap-1.5 text-sm font-medium text-gray-900 dark:text-white">
                <span class="flex h-6 w-6 items-center justify-center rounded-md" :class="item.iconBg">
                  <Icon :name="item.icon" size="xs" :class="item.iconClass" />
                </span>
                {{ item.name }}
              </p>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ item.desc }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const items = computed(() => [
  {
    image: '/official-group-qq.jpg',
    alt: t('dashboard.contactQr.groupQrAlt'),
    icon: 'users' as const,
    iconBg: 'bg-primary-100 dark:bg-primary-900/30',
    iconClass: 'text-primary-600 dark:text-primary-400',
    name: t('dashboard.contactQr.group'),
    desc: t('dashboard.contactQr.groupDesc'),
  },
  {
    image: '/customer-service-qq.jpg',
    alt: t('dashboard.contactQr.supportQrAlt'),
    icon: 'chat' as const,
    iconBg: 'bg-sky-100 dark:bg-sky-900/30',
    iconClass: 'text-sky-600 dark:text-sky-400',
    name: t('dashboard.contactQr.support'),
    desc: t('dashboard.contactQr.supportDesc'),
  },
])
</script>
