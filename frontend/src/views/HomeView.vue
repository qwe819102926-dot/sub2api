<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Compact Home Page -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="compact-home flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="compact-home-header border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <button
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img
          :src="siteLogo || '/logo.svg'"
          alt="Logo"
          class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
        />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>

  <!-- Default Home Page -->
  <div
    v-else
    class="home-page relative flex min-h-screen flex-col overflow-hidden bg-[#edf0ed] text-gray-950 dark:bg-[#030706] dark:text-white"
  >
    <!-- A restrained technical grid keeps the page visual without competing with the product. -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div class="home-grid absolute inset-0"></div>
      <div class="home-grid-fade absolute inset-x-0 top-0 h-[32rem]"></div>
    </div>

    <!-- Header -->
    <header class="relative z-20 border-b border-gray-900/10 px-5 py-4 dark:border-white/10 sm:px-8">
      <nav class="mx-auto flex max-w-6xl items-center justify-between gap-4">
        <!-- Logo -->
        <div class="flex min-w-0 items-center gap-3">
          <div class="h-9 w-9 shrink-0 overflow-hidden rounded-lg border border-gray-900/10 bg-white dark:border-white/10 dark:bg-white/5">
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <span class="truncate text-sm font-semibold text-gray-950 dark:text-white">{{ siteName }}</span>
        </div>

        <!-- Nav Actions -->
        <div class="flex shrink-0 items-center gap-1.5 sm:gap-2">
          <!-- Language Switcher -->
          <LocaleSwitcher />

          <!-- Doc Link -->
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="rounded-md p-2 text-gray-500 transition-colors hover:bg-gray-900/5 hover:text-gray-950 dark:text-dark-400 dark:hover:bg-white/10 dark:hover:text-white"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>

          <!-- Theme Toggle -->
          <button
            @click="toggleTheme"
            class="rounded-md p-2 text-gray-500 transition-colors hover:bg-gray-900/5 hover:text-gray-950 dark:text-dark-400 dark:hover:bg-white/10 dark:hover:text-white"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>

          <!-- Login / Dashboard Button -->
          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="inline-flex items-center gap-1.5 rounded-md bg-gray-950 py-1.5 pl-1.5 pr-2.5 transition-colors hover:bg-gray-800 dark:bg-white dark:hover:bg-gray-200"
          >
            <span
              class="flex h-5 w-5 items-center justify-center rounded-sm bg-primary-500 text-[10px] font-semibold text-white"
            >
              {{ userInitial }}
            </span>
            <span class="text-xs font-medium text-white dark:text-gray-950">{{ t('home.dashboard') }}</span>
            <svg
              class="h-3 w-3 text-gray-400 dark:text-gray-500"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M4.5 19.5l15-15m0 0H8.25m11.25 0v11.25"
              />
            </svg>
          </router-link>
          <router-link
            v-else
            to="/login"
            class="inline-flex items-center rounded-md bg-gray-950 px-3.5 py-2 text-xs font-medium text-white transition-colors hover:bg-gray-800 dark:bg-white dark:text-gray-950 dark:hover:bg-gray-200"
          >
            {{ t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <!-- Main Content -->
    <main class="relative z-10 flex-1 px-5 pb-16 pt-14 sm:px-8 sm:pt-20">
      <div class="mx-auto max-w-6xl">
        <!-- Hero -->
        <section class="hero-stage mx-auto mb-14 max-w-6xl text-center sm:mb-20">
          <div class="mb-5 inline-flex items-center gap-2 border border-primary-500/25 bg-primary-500/5 px-3 py-1.5 text-xs font-medium text-primary-700 dark:bg-primary-400/10 dark:text-primary-300">
            <span class="h-1.5 w-1.5 bg-primary-500"></span>
            API INFRASTRUCTURE
          </div>
          <div>
            <h1
              class="hero-copy-title mx-auto max-w-4xl text-4xl font-semibold leading-[1.08] sm:text-5xl lg:text-6xl"
            >
              {{ siteName }}
            </h1>
            <p class="hero-copy-lede mx-auto mt-5 max-w-2xl text-base leading-7 sm:text-lg">
              {{ siteSubtitle }}
            </p>

            <!-- CTA Button -->
            <div class="mt-8">
              <router-link
                :to="isAuthenticated ? dashboardPath : '/login'"
                class="hero-action inline-flex items-center gap-2 px-5 py-3 text-sm font-semibold transition-colors"
              >
                {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
                <Icon name="arrowRight" size="sm" :stroke-width="2" />
              </router-link>
            </div>
          </div>

          <!-- Product request preview -->
          <section class="hero-console">
          <div class="terminal-container mx-auto">
            <div class="terminal-window">
              <div class="terminal-header">
                <div class="terminal-buttons" aria-hidden="true">
                  <span class="btn-close"></span>
                  <span class="btn-minimize"></span>
                  <span class="btn-maximize"></span>
                </div>
                <span class="terminal-title">api request</span>
                <span class="terminal-status"><span></span> live</span>
              </div>
              <div class="terminal-body">
                <div class="code-line line-1">
                  <span class="code-prompt">$</span>
                  <span class="code-cmd">curl</span>
                  <span class="code-flag">-X POST</span>
                  <span class="code-url">/v1/messages</span>
                </div>
                <div class="code-line line-2">
                  <span class="code-comment"># Routing request to the optimal upstream</span>
                </div>
                <div class="code-line line-3">
                  <span class="code-success">200 OK</span>
                  <span class="code-response">{ "content": "Hello!" }</span>
                </div>
                <div class="code-line line-4">
                  <span class="code-prompt">$</span>
                  <span class="cursor"></span>
                </div>
              </div>
              <div class="terminal-metrics">
                <div><span>UPSTREAM</span><strong>Claude</strong></div>
                <div><span>LATENCY</span><strong>284 ms</strong></div>
                <div><span>USAGE</span><strong>0.0021</strong></div>
              </div>
            </div>
          </div>
          </section>
        </section>

        <!-- Feature Tags - Centered -->
        <div class="signal-strip mb-14 grid py-1 sm:mb-20 sm:grid-cols-3">
          <div
            class="flex items-center justify-center gap-2 border-b border-gray-900/10 px-4 py-4 last:border-b-0 dark:border-white/10 sm:border-b-0 sm:border-r sm:last:border-r-0"
          >
            <Icon name="swap" size="sm" class="text-primary-500" />
            <span class="text-sm font-medium text-gray-700 dark:text-dark-200">{{
              t('home.tags.subscriptionToApi')
            }}</span>
          </div>
          <div
            class="flex items-center justify-center gap-2 border-b border-gray-900/10 px-4 py-4 last:border-b-0 dark:border-white/10 sm:border-b-0 sm:border-r sm:last:border-r-0"
          >
            <Icon name="shield" size="sm" class="text-primary-500" />
            <span class="text-sm font-medium text-gray-700 dark:text-dark-200">{{
              t('home.tags.stickySession')
            }}</span>
          </div>
          <div
            class="flex items-center justify-center gap-2 px-4 py-4"
          >
            <Icon name="chart" size="sm" class="text-primary-500" />
            <span class="text-sm font-medium text-gray-700 dark:text-dark-200">{{
              t('home.tags.realtimeBilling')
            }}</span>
          </div>
        </div>

        <!-- Features Grid -->
        <div class="capability-grid mb-16 grid gap-px overflow-hidden md:grid-cols-3 sm:mb-20">
          <!-- Feature 1: Unified Gateway -->
          <div
            class="group bg-[#f7f8f6] p-6 transition-colors duration-200 hover:bg-white dark:bg-[#090d0d] dark:hover:bg-white/[0.03] sm:p-8"
          >
            <span class="feature-index">01</span>
            <div
              class="mb-8 flex h-10 w-10 items-center justify-center border border-blue-500/25 bg-blue-500/10 text-blue-700 dark:text-blue-300"
            >
              <Icon name="server" size="lg" />
            </div>
            <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('home.features.unifiedGateway') }}
            </h3>
            <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-400">
              {{ t('home.features.unifiedGatewayDesc') }}
            </p>
          </div>

          <!-- Feature 2: Account Pool -->
          <div
            class="group bg-[#f7f8f6] p-6 transition-colors duration-200 hover:bg-white dark:bg-[#090d0d] dark:hover:bg-white/[0.03] sm:p-8"
          >
            <span class="feature-index">02</span>
            <div
              class="mb-8 flex h-10 w-10 items-center justify-center border border-primary-500/25 bg-primary-500/10 text-primary-700 dark:text-primary-300"
            >
              <Icon name="users" size="lg" />
            </div>
            <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('home.features.multiAccount') }}
            </h3>
            <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-400">
              {{ t('home.features.multiAccountDesc') }}
            </p>
          </div>

          <!-- Feature 3: Billing & Quota -->
          <div
            class="group bg-[#f7f8f6] p-6 transition-colors duration-200 hover:bg-white dark:bg-[#090d0d] dark:hover:bg-white/[0.03] sm:p-8"
          >
            <span class="feature-index">03</span>
            <div
              class="mb-8 flex h-10 w-10 items-center justify-center border border-amber-500/25 bg-amber-500/10 text-amber-700 dark:text-amber-300"
            >
              <svg
                class="h-6 w-6"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="1.5"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z"
                />
              </svg>
            </div>
            <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('home.features.balanceQuota') }}
            </h3>
            <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-400">
              {{ t('home.features.balanceQuotaDesc') }}
            </p>
          </div>
        </div>

        <!-- Supported Providers -->
        <div class="mb-8 text-center">
          <p class="mb-3 text-xs font-semibold uppercase text-primary-700 dark:text-primary-300">Models</p>
          <h2 class="mb-3 text-2xl font-semibold text-gray-900 dark:text-white">
            {{ t('home.providers.title') }}
          </h2>
          <p class="text-sm text-gray-600 dark:text-dark-400">
            {{ t('home.providers.description') }}
          </p>
        </div>

        <div class="mb-16 flex flex-wrap items-center justify-center gap-2 sm:mb-20">
          <!-- Claude - Supported -->
          <div
            class="flex items-center gap-2 border border-gray-900/10 bg-white/60 px-4 py-3 dark:border-white/10 dark:bg-white/[0.03]"
          >
            <div
              class="flex h-8 w-8 items-center justify-center bg-orange-500"
            >
              <span class="text-xs font-bold text-white">C</span>
            </div>
            <span class="text-sm font-medium text-gray-700 dark:text-dark-200">{{ t('home.providers.claude') }}</span>
            <span
              class="border border-primary-500/25 bg-primary-500/10 px-1.5 py-0.5 text-[10px] font-medium text-primary-700 dark:text-primary-300"
              >{{ t('home.providers.supported') }}</span
            >
          </div>
          <!-- GPT - Supported -->
          <div
            class="flex items-center gap-2 border border-gray-900/10 bg-white/60 px-4 py-3 dark:border-white/10 dark:bg-white/[0.03]"
          >
            <div
              class="flex h-8 w-8 items-center justify-center bg-emerald-600"
            >
              <span class="text-xs font-bold text-white">G</span>
            </div>
            <span class="text-sm font-medium text-gray-700 dark:text-dark-200">GPT</span>
            <span
              class="border border-primary-500/25 bg-primary-500/10 px-1.5 py-0.5 text-[10px] font-medium text-primary-700 dark:text-primary-300"
              >{{ t('home.providers.supported') }}</span
            >
          </div>
          <!-- Gemini - Supported -->
          <div
            class="flex items-center gap-2 border border-gray-900/10 bg-white/60 px-4 py-3 dark:border-white/10 dark:bg-white/[0.03]"
          >
            <div
              class="flex h-8 w-8 items-center justify-center bg-blue-600"
            >
              <span class="text-xs font-bold text-white">G</span>
            </div>
            <span class="text-sm font-medium text-gray-700 dark:text-dark-200">{{ t('home.providers.gemini') }}</span>
            <span
              class="border border-primary-500/25 bg-primary-500/10 px-1.5 py-0.5 text-[10px] font-medium text-primary-700 dark:text-primary-300"
              >{{ t('home.providers.supported') }}</span
            >
          </div>
          <!-- Antigravity - Supported -->
          <div
            class="flex items-center gap-2 border border-gray-900/10 bg-white/60 px-4 py-3 dark:border-white/10 dark:bg-white/[0.03]"
          >
            <div
              class="flex h-8 w-8 items-center justify-center bg-rose-600"
            >
              <span class="text-xs font-bold text-white">A</span>
            </div>
            <span class="text-sm font-medium text-gray-700 dark:text-dark-200">{{ t('home.providers.antigravity') }}</span>
            <span
              class="border border-primary-500/25 bg-primary-500/10 px-1.5 py-0.5 text-[10px] font-medium text-primary-700 dark:text-primary-300"
              >{{ t('home.providers.supported') }}</span
            >
          </div>
          <!-- More - Coming Soon -->
          <div
            class="flex items-center gap-2 border border-gray-900/10 bg-white/40 px-4 py-3 opacity-60 dark:border-white/10 dark:bg-white/[0.03]"
          >
            <div
              class="flex h-8 w-8 items-center justify-center bg-gray-600"
            >
              <span class="text-xs font-bold text-white">+</span>
            </div>
            <span class="text-sm font-medium text-gray-700 dark:text-dark-200">{{ t('home.providers.more') }}</span>
            <span
              class="rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-500 dark:bg-dark-700 dark:text-dark-400"
              >{{ t('home.providers.soon') }}</span
            >
          </div>
        </div>
      </div>
    </main>

    <!-- Footer -->
    <footer class="site-footer relative z-10 px-5 py-8 sm:px-8">
      <div
        class="mx-auto flex max-w-6xl flex-col items-center justify-between gap-4 text-center sm:flex-row sm:text-left"
      >
        <p class="text-sm text-gray-500 dark:text-dark-400">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="flex items-center gap-4">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-primary-700 dark:text-dark-400 dark:hover:text-primary-300"
          >
            {{ t('home.docs') }}
          </a>
          <a
            :href="githubUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-primary-700 dark:text-dark-400 dark:hover:text-primary-300"
          >
            GitHub
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Key 智中转')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || '一个密钥，统一接入多模型 API')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// GitHub URL
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// Toggle theme
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

// Initialize theme
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()

  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
.home-grid {
  background-image:
    linear-gradient(rgba(15, 23, 42, 0.055) 1px, transparent 1px),
    linear-gradient(90deg, rgba(15, 23, 42, 0.055) 1px, transparent 1px);
  background-size: 48px 48px;
}

.home-grid-fade {
  background: linear-gradient(180deg, rgba(237, 240, 237, 0) 0%, #edf0ed 100%);
}

.dark .home-grid {
  background-image:
    linear-gradient(rgba(255, 255, 255, 0.055) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255, 255, 255, 0.055) 1px, transparent 1px);
}

.dark .home-grid-fade {
  background: linear-gradient(180deg, rgba(3, 7, 6, 0) 0%, #030706 100%);
}

.hero-stage {
  position: relative;
  overflow: hidden;
  padding: 4.75rem 3rem 0;
  background: #eff6ff;
  border: 1px solid rgba(37, 99, 235, 0.16);
  box-shadow: 0 32px 72px -46px rgba(30, 64, 175, 0.3);
}

.hero-stage::before {
  position: absolute;
  inset: 20px;
  content: '';
  border: 1px solid rgba(37, 99, 235, 0.09);
  pointer-events: none;
}

.hero-stage > * {
  position: relative;
  z-index: 1;
}

.hero-stage .inline-flex {
  border-color: rgba(37, 99, 235, 0.2);
  background: rgba(37, 99, 235, 0.06);
  color: #2563eb;
}

.hero-copy-title {
  color: #172554;
}

.hero-copy-lede {
  color: #52627f;
}

.hero-action {
  background: #2563eb;
  color: #f8fbff;
  box-shadow: 6px 6px 0 #c9dcff;
}

.hero-action:hover {
  background: #1d4ed8;
  box-shadow: 3px 3px 0 #c9dcff;
  transform: translate(3px, 3px);
}

.hero-console {
  margin: 3.75rem -3rem 0;
  padding: 0 3rem;
  border-top: 1px solid rgba(37, 99, 235, 0.1);
}

.dark .hero-stage {
  background: #13254a;
  border-color: rgba(147, 197, 253, 0.24);
  box-shadow: 0 32px 72px -46px rgba(0, 0, 0, 0.65);
}

.dark .hero-stage::before {
  border-color: rgba(255, 255, 255, 0.1);
}

.dark .hero-stage .inline-flex {
  border-color: rgba(147, 197, 253, 0.3);
  background: rgba(96, 165, 250, 0.12);
  color: #bfdbfe;
}

.dark .hero-copy-title {
  color: #f5fffc;
}

.dark .hero-copy-lede {
  color: #bfcae1;
}

.dark .hero-console {
  border-top-color: rgba(255, 255, 255, 0.1);
}

.signal-strip {
  border-top: 1px solid rgba(15, 23, 42, 0.14);
  border-bottom: 1px solid rgba(15, 23, 42, 0.14);
}

.dark .signal-strip {
  border-color: rgba(255, 255, 255, 0.12);
}

.capability-grid {
  border: 1px solid rgba(15, 23, 42, 0.16);
  background: rgba(15, 23, 42, 0.16);
}

.dark .capability-grid {
  border-color: rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.12);
}

.capability-grid > div {
  position: relative;
}

.feature-index {
  position: absolute;
  right: 1.75rem;
  top: 1.75rem;
  color: rgba(15, 23, 42, 0.28);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.75rem;
}

.dark .feature-index {
  color: rgba(255, 255, 255, 0.3);
}

.site-footer {
  background: #f8fbff;
  border-top: 1px solid rgba(37, 99, 235, 0.12);
}

.site-footer :is(p, a) {
  color: #52627f;
}

.site-footer a:hover {
  color: #2563eb;
}

.dark .site-footer {
  background: #0d1a34;
  border-top-color: rgba(147, 197, 253, 0.16);
}

.dark .site-footer :is(p, a) {
  color: #a7bbb6;
}

.dark .site-footer a:hover {
  color: #bfdbfe;
}

/* Terminal Container */
.terminal-container {
  position: relative;
  display: block;
  max-width: 58rem;
}

/* Terminal Window */
.terminal-window {
  width: 100%;
  background: #eef5ff;
  border-radius: 0;
  box-shadow:
    0 26px 56px -34px rgba(30, 64, 175, 0.28),
    0 0 0 1px rgba(37, 99, 235, 0.18),
    inset 0 1px 0 rgba(255, 255, 255, 0.08);
  overflow: hidden;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.terminal-window:hover {
  transform: translateY(-5px);
  box-shadow:
    0 28px 64px -28px rgba(15, 23, 42, 0.6),
    0 0 0 1px rgba(20, 184, 166, 0.4),
    inset 0 1px 0 rgba(255, 255, 255, 0.08);
}

/* Terminal Header */
.terminal-header {
  display: flex;
  align-items: center;
  padding: 11px 14px;
  background: #dbeafe;
  border-bottom: 1px solid rgba(37, 99, 235, 0.16);
}

.terminal-buttons {
  display: flex;
  gap: 8px;
}

.terminal-buttons span {
  width: 8px;
  height: 8px;
  border-radius: 2px;
}

.btn-close {
  background: #ef4444;
}
.btn-minimize {
  background: #eab308;
}
.btn-maximize {
  background: #22c55e;
}

.terminal-title {
  flex: 1;
  text-align: center;
  font-size: 11px;
  font-family: ui-monospace, monospace;
  color: #48658f;
  margin-right: 44px;
  text-transform: uppercase;
}

.terminal-status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: #48658f;
  font-family: ui-monospace, monospace;
  font-size: 10px;
  text-transform: uppercase;
}

.terminal-status span {
  width: 6px;
  height: 6px;
  background: #2563eb;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.12);
}

/* Terminal Body */
.terminal-body {
  min-height: 13rem;
  padding: 32px 34px;
  font-family: ui-monospace, 'Fira Code', monospace;
  font-size: 13px;
  line-height: 2;
}

.code-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  opacity: 0;
  animation: line-appear 0.5s ease forwards;
}

.line-1 {
  animation-delay: 0.3s;
}
.line-2 {
  animation-delay: 1s;
}
.line-3 {
  animation-delay: 1.8s;
}
.line-4 {
  animation-delay: 2.5s;
}

@keyframes line-appear {
  from {
    opacity: 0;
    transform: translateY(5px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.code-prompt {
  color: #2563eb;
  font-weight: bold;
}
.code-cmd {
  color: #1e3a8a;
}
.code-flag {
  color: #7c3aed;
}
.code-url {
  color: #0284c7;
}
.code-comment {
  color: #64748b;
  font-style: italic;
}
.code-success {
  color: #166534;
  background: #dcfce7;
  padding: 2px 8px;
  border-radius: 2px;
  font-weight: 600;
}
.code-response {
  color: #a16207;
}

.terminal-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border-top: 1px solid rgba(37, 99, 235, 0.13);
  background: #f8fbff;
}

.terminal-metrics > div {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 5px;
  padding: 14px 18px;
  border-right: 1px solid rgba(37, 99, 235, 0.13);
}

.terminal-metrics > div:last-child {
  border-right: 0;
}

.terminal-metrics span {
  color: #71809a;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 10px;
}

.terminal-metrics strong {
  overflow: hidden;
  color: #1e3a8a;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.terminal-metrics > div:nth-child(2) strong {
  color: #b45309;
}

.terminal-metrics > div:nth-child(3) strong {
  color: #2563eb;
}

/* Blinking Cursor */
.cursor {
  display: inline-block;
  width: 8px;
  height: 16px;
  background: #22c55e;
  animation: blink 1s step-end infinite;
}

@keyframes blink {
  0%,
  50% {
    opacity: 1;
  }
  51%,
  100% {
    opacity: 0;
  }
}

/* Dark mode adjustments */
:deep(.dark) .terminal-window {
  box-shadow:
    0 24px 56px -28px rgba(0, 0, 0, 0.7),
    0 0 0 1px rgba(20, 184, 166, 0.2),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
}

@media (max-width: 640px) {
  .hero-stage {
    padding: 3.5rem 1.25rem 0;
  }

  .hero-stage::before {
    inset: 12px;
  }

  .hero-console {
    margin: 3rem -1.25rem 0;
    padding: 0 1.25rem;
  }

  .terminal-body {
    min-height: 10rem;
    padding: 20px;
    font-size: 11px;
  }

  .terminal-title {
    margin-right: 22px;
  }

  .terminal-status {
    display: none;
  }

  .terminal-metrics > div {
    padding: 12px 10px;
  }

  .terminal-metrics span {
    font-size: 9px;
  }

  .terminal-metrics strong {
    font-size: 10px;
  }
}
</style>

<style scoped>
/* Warm, hand-painted treatment for the public entry page. */
.compact-home {
  background:
    radial-gradient(circle at 12% 8%, rgba(133, 205, 202, 0.24), transparent 18rem),
    radial-gradient(circle at 88% 86%, rgba(232, 168, 124, 0.2), transparent 17rem),
    #f4e4bc !important;
  color: #3a3226 !important;
}

.compact-home-header {
  border-color: rgba(124, 185, 168, 0.38) !important;
  background: rgba(250, 245, 235, 0.72);
}

.compact-home :deep(a.rounded-lg),
.compact-home :deep(button.rounded-lg) {
  border-radius: 16px;
  transition: transform 600ms ease-in-out, background-color 600ms ease-in-out;
}

.compact-home :deep(a.rounded-lg:hover),
.compact-home :deep(button.rounded-lg:hover) {
  transform: translateY(-1px) rotate(-0.2deg);
}

.home-page {
  background:
    radial-gradient(circle at 12% 3%, rgba(133, 205, 202, 0.26), transparent 22rem),
    radial-gradient(circle at 90% 16%, rgba(232, 168, 124, 0.22), transparent 18rem),
    #f4e4bc !important;
  color: #3a3226 !important;
}

.home-grid {
  background-image:
    radial-gradient(ellipse at 20% 28%, rgba(124, 185, 168, 0.18) 0 1px, transparent 1.5px),
    radial-gradient(ellipse at 72% 62%, rgba(195, 141, 148, 0.13) 0 1px, transparent 1.5px) !important;
  background-size: 34px 34px, 48px 48px !important;
}

.home-grid::before,
.home-grid::after {
  position: absolute;
  content: '';
  border-radius: 999px;
  filter: blur(1px);
}

.home-grid::before {
  top: 5.5rem;
  left: 6%;
  width: 12rem;
  height: 4.8rem;
  background:
    radial-gradient(ellipse at 18% 70%, rgba(250, 245, 235, 0.85) 0 28%, transparent 29%),
    radial-gradient(ellipse at 42% 40%, rgba(250, 245, 235, 0.9) 0 33%, transparent 34%),
    radial-gradient(ellipse at 68% 58%, rgba(250, 245, 235, 0.86) 0 31%, transparent 32%);
}

.home-grid::after {
  right: 5%;
  bottom: 14rem;
  width: 13rem;
  height: 5.5rem;
  opacity: 0.72;
  background:
    radial-gradient(ellipse at 20% 70%, rgba(250, 245, 235, 0.78) 0 27%, transparent 28%),
    radial-gradient(ellipse at 46% 38%, rgba(250, 245, 235, 0.84) 0 34%, transparent 35%),
    radial-gradient(ellipse at 72% 60%, rgba(250, 245, 235, 0.8) 0 30%, transparent 31%);
}

.home-grid-fade {
  background: linear-gradient(180deg, rgba(244, 228, 188, 0) 0%, #f4e4bc 100%) !important;
}

.home-page > header {
  border-color: rgba(124, 185, 168, 0.36) !important;
  background: rgba(250, 245, 235, 0.66);
  backdrop-filter: blur(10px);
}

.home-page > header :deep(.rounded-md) {
  border-radius: 14px;
  transition: transform 600ms ease-in-out, background-color 600ms ease-in-out;
}

.home-page > header :deep(.rounded-md:hover) {
  background: rgba(124, 185, 168, 0.18) !important;
  transform: translateY(-1px) rotate(-0.2deg);
}

.home-page > header :deep(.bg-gray-950) {
  background: #639d8c !important;
}

.hero-stage {
  overflow: visible;
  padding: 4.5rem 3rem 0;
  border: 1px solid rgba(124, 185, 168, 0.48);
  border-radius: 28px;
  background:
    radial-gradient(circle at 88% 13%, rgba(133, 205, 202, 0.28), transparent 12rem),
    radial-gradient(circle at 12% 85%, rgba(232, 168, 124, 0.18), transparent 14rem),
    #faf5eb;
  box-shadow: 0 10px 32px rgba(124, 185, 168, 0.24);
}

.hero-stage::before {
  inset: 14px;
  border: 1px dashed rgba(124, 185, 168, 0.35);
  border-radius: 20px;
}

.hero-stage::after {
  position: absolute;
  top: -1.4rem;
  right: 5%;
  width: 4.25rem;
  height: 2.25rem;
  content: '';
  border-radius: 999px;
  background:
    radial-gradient(ellipse at 20% 70%, #fffaf0 0 27%, transparent 28%),
    radial-gradient(ellipse at 46% 38%, #fffaf0 0 34%, transparent 35%),
    radial-gradient(ellipse at 72% 60%, #fffaf0 0 30%, transparent 31%);
  box-shadow: 0 5px 16px rgba(124, 185, 168, 0.12);
}

.hero-stage .inline-flex {
  border-color: rgba(124, 185, 168, 0.45) !important;
  border-radius: 999px;
  background: rgba(124, 185, 168, 0.16) !important;
  color: #3c685b !important;
}

.hero-copy-title {
  color: #3a3226 !important;
}

.hero-copy-lede {
  color: #5a4a3a !important;
}

.hero-action {
  border-radius: 16px;
  background: #639d8c !important;
  color: #fffaf0 !important;
  box-shadow: 0 4px 16px rgba(124, 185, 168, 0.32) !important;
  transition: transform 600ms ease-in-out, background-color 600ms ease-in-out, box-shadow 600ms ease-in-out;
}

.hero-action:hover {
  background: #4d8373 !important;
  box-shadow: 0 8px 24px rgba(124, 185, 168, 0.4) !important;
  transform: translateY(-2px) rotate(-0.3deg) scale(1.02) !important;
}

.hero-console {
  margin: 3.4rem -3rem 0;
  border-top-color: rgba(124, 185, 168, 0.28);
}

.terminal-window {
  border-radius: 20px 20px 0 0;
  background: #faf5eb;
  box-shadow: 0 8px 28px rgba(124, 185, 168, 0.22), 0 0 0 1px rgba(124, 185, 168, 0.36);
  transition: transform 600ms ease-in-out, box-shadow 600ms ease-in-out;
}

.terminal-window:hover {
  box-shadow: 0 12px 30px rgba(124, 185, 168, 0.3), 0 0 0 1px rgba(124, 185, 168, 0.5);
  transform: translateY(-2px) rotate(0.15deg);
}

.terminal-header {
  border-bottom-color: rgba(124, 185, 168, 0.32);
  background: #dceee6;
}

.terminal-buttons span {
  border-radius: 50%;
}

.btn-close { background: #c38d94; }
.btn-minimize { background: #e8c07a; }
.btn-maximize { background: #7cb9a8; }
.terminal-title,
.terminal-status { color: #5a4a3a; }
.terminal-status span { background: #7cb9a8; box-shadow: 0 0 0 3px rgba(124, 185, 168, 0.18); }
.terminal-body { color: #5a4a3a; }
.code-prompt, .code-url { color: #4d8373; }
.code-cmd { color: #3a3226; }
.code-flag { color: #9d754e; }
.code-comment { color: #8a7a6a; }
.code-success { color: #3c685b; background: #dceee6; border-radius: 10px; }
.code-response { color: #9d754e; }
.cursor { background: #7cb9a8; animation-duration: 2.4s; }

.terminal-metrics {
  border-top-color: rgba(124, 185, 168, 0.28);
  background: #fffaf0;
}

.terminal-metrics > div { border-right-color: rgba(124, 185, 168, 0.24); }
.terminal-metrics span { color: #8a7a6a; }
.terminal-metrics strong, .terminal-metrics > div:nth-child(2) strong, .terminal-metrics > div:nth-child(3) strong { color: #3a3226; }

.signal-strip {
  border-color: rgba(124, 185, 168, 0.42);
  border-radius: 18px;
  background: rgba(250, 245, 235, 0.68);
}

.capability-grid {
  gap: 1.25rem;
  border: 0;
  background: transparent;
  overflow: visible;
}

.capability-grid > div {
  border: 1px solid rgba(124, 185, 168, 0.4);
  border-radius: 18px;
  background: rgba(250, 245, 235, 0.92) !important;
  box-shadow: 0 4px 16px rgba(124, 185, 168, 0.18);
  transition: transform 600ms ease-in-out, box-shadow 600ms ease-in-out, background-color 600ms ease-in-out !important;
}

.capability-grid > div:hover {
  background: #fffaf0 !important;
  box-shadow: 0 8px 24px rgba(124, 185, 168, 0.28);
  transform: translateY(-3px) rotate(0.25deg);
}

.feature-index { color: rgba(90, 74, 58, 0.3); }

.site-footer {
  border-top-color: rgba(124, 185, 168, 0.38);
  background: rgba(220, 238, 230, 0.62);
}

.site-footer :is(p, a) { color: #5a4a3a !important; }
.site-footer a:hover { color: #3c685b !important; }

.dark .home-page,
.dark .compact-home { background: #1d251c !important; color: #f4e4bc !important; }
.dark .home-grid-fade { background: linear-gradient(180deg, rgba(29, 37, 28, 0) 0%, #1d251c 100%) !important; }
.dark .home-page > header, .dark .compact-home-header { background: rgba(39, 48, 37, 0.82); border-color: rgba(184, 209, 188, 0.2) !important; }
.dark .hero-stage { background: #333b30; border-color: rgba(184, 209, 188, 0.28); box-shadow: 0 12px 32px rgba(0, 0, 0, 0.32); }
.dark .hero-stage::before { border-color: rgba(184, 209, 188, 0.2); }
.dark .hero-copy-title { color: #f4e4bc !important; }
.dark .hero-copy-lede { color: #d6ddcf !important; }
.dark .terminal-window { background: #333b30; }
.dark .terminal-header { background: #414a3c; }
.dark .terminal-body, .dark .terminal-title, .dark .terminal-status { color: #e7ecd9; }
.dark .terminal-metrics { background: #273025; }
.dark .capability-grid > div { background: #333b30 !important; border-color: rgba(184, 209, 188, 0.24); }
.dark .capability-grid > div:hover { background: #414a3c !important; }
.dark .signal-strip, .dark .site-footer { background: rgba(51, 59, 48, 0.72); border-color: rgba(184, 209, 188, 0.2); }
.dark .site-footer :is(p, a) { color: #d6ddcf !important; }

@media (prefers-reduced-motion: reduce) {
  .compact-home :deep(a.rounded-lg),
  .compact-home :deep(button.rounded-lg),
  .home-page > header :deep(.rounded-md),
  .hero-action,
  .terminal-window,
  .capability-grid > div,
  .sidebar-link {
    transition-duration: 1ms !important;
    transform: none !important;
  }

  .code-line,
  .cursor { animation: none !important; opacity: 1; }
}

@media (max-width: 640px) {
  .hero-stage { padding: 3.5rem 1.25rem 0; border-radius: 22px; }
  .hero-console { margin: 3rem -1.25rem 0; }
  .home-grid::after { display: none; }
}
</style>
