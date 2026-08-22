<template>
  <div class="auth-shell relative flex min-h-screen items-center justify-center overflow-hidden p-4 sm:p-8">
    <div class="auth-grid pointer-events-none absolute inset-0"></div>

    <div class="auth-frame relative z-10 grid w-full max-w-6xl overflow-hidden lg:grid-cols-[1.18fr_0.82fr]">
      <section class="auth-brand-panel hidden flex-col justify-between p-10 lg:flex">
        <div>
          <div class="mb-8 flex h-12 w-12 items-center justify-center overflow-hidden border border-primary-300/30 bg-white/5">
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <p class="mb-5 text-xs font-semibold text-primary-300">API CONTROL PLANE</p>
          <h1 class="max-w-sm text-4xl font-semibold leading-tight text-white">{{ siteName }}</h1>
          <p class="mt-5 max-w-sm text-sm leading-6 text-emerald-50/65">{{ siteSubtitle }}</p>
        </div>
        <div class="border-t border-white/10 pt-5 text-xs text-emerald-50/45">
          Secure access for your API workspace
        </div>
      </section>

      <section class="auth-form-panel flex min-h-[34rem] items-center px-6 py-10 sm:px-10 lg:px-14">
        <div class="mx-auto w-full max-w-md">
          <div class="mb-8 lg:hidden">
            <div class="mb-4 flex h-11 w-11 items-center justify-center overflow-hidden border border-gray-900/10 bg-white dark:border-white/10 dark:bg-white/5">
              <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
            </div>
            <p class="text-sm font-semibold text-gray-950 dark:text-white">{{ siteName }}</p>
          </div>

          <slot />

          <div class="mt-7 text-center text-sm">
            <slot name="footer" />
          </div>

          <div class="mt-8 text-center text-xs text-gray-400 dark:text-dark-500">
            &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || 'Key 智中转')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform')
const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.auth-shell {
  background:
    radial-gradient(circle at 15% 10%, rgba(133, 205, 202, 0.3), transparent 20rem),
    radial-gradient(circle at 88% 85%, rgba(232, 168, 124, 0.26), transparent 18rem),
    #f4e4bc;
}

.auth-grid {
  background-image:
    radial-gradient(ellipse at 20% 40%, rgba(124, 185, 168, 0.2) 0 1px, transparent 1.5px),
    radial-gradient(ellipse at 70% 65%, rgba(195, 141, 148, 0.15) 0 1px, transparent 1.5px);
  background-size: 34px 34px, 46px 46px;
}

.auth-frame {
  border: 1px solid rgba(124, 185, 168, 0.44);
  border-radius: 24px;
  box-shadow: 0 12px 40px rgba(94, 122, 96, 0.2);
}

.auth-brand-panel {
  background:
    radial-gradient(circle at 78% 13%, rgba(244, 228, 188, 0.24) 0 3.6rem, transparent 3.7rem),
    radial-gradient(ellipse at 18% 84%, rgba(133, 205, 202, 0.24) 0 5rem, transparent 5.1rem),
    #567d70;
  background-size: auto;
}

.auth-form-panel {
  background: rgba(250, 245, 235, 0.97);
}

.auth-form-panel :deep(.btn-primary) {
  background: #639d8c;
  box-shadow: 0 4px 16px rgba(124, 185, 168, 0.28);
}

.auth-form-panel :deep(.btn-primary:hover) {
  background: #4d8373;
  transform: translateY(-1px) rotate(-0.2deg) scale(1.02);
}

.auth-form-panel :deep(.input) {
  border-radius: 16px;
}

.dark .auth-shell {
  background: #1d251c;
}

.dark .auth-grid {
  background-image:
    radial-gradient(ellipse at 20% 40%, rgba(244, 228, 188, 0.12) 0 1px, transparent 1.5px),
    radial-gradient(ellipse at 70% 65%, rgba(133, 205, 202, 0.12) 0 1px, transparent 1.5px);
}

.dark .auth-frame {
  border-color: rgba(184, 209, 188, 0.23);
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.42);
}

.dark .auth-form-panel {
  background: #333b30;
}
</style>
