<template>
  <div class="card">
    <div class="flex flex-col gap-6 p-6 md:flex-row md:items-center md:justify-between">
      <div class="min-w-0">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('dashboard.contactQr.title') }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('dashboard.contactQr.subtitle') }}</p>
      </div>
      <div class="flex flex-col gap-6 sm:flex-row sm:gap-10">
        <div v-for="item in items" :key="item.name" class="flex items-center gap-4">
          <img :src="item.image" :alt="item.alt" class="h-28 w-28 flex-shrink-0 rounded-xl border border-gray-100 bg-white dark:border-dark-700" />
          <div class="min-w-0">
            <p class="flex items-center gap-2 text-sm font-medium text-gray-900 dark:text-white">
              <span class="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-lg" :class="item.iconBg">
                <Icon :name="item.icon" size="sm" :class="item.iconClass" />
              </span>
              {{ item.name }}
            </p>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ item.desc }}</p>
          </div>
        </div>
      </div>
    </div>
  </div>
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
    icon: 'headphones' as const,
    iconBg: 'bg-sky-100 dark:bg-sky-900/30',
    iconClass: 'text-sky-600 dark:text-sky-400',
    name: t('dashboard.contactQr.support'),
    desc: t('dashboard.contactQr.supportDesc'),
  },
])
</script>
