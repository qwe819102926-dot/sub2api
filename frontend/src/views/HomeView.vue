<template>
  <div v-if="hasHomeContent" class="min-h-screen">
    <iframe v-if="isHomeContentUrl" :src="homeContent.trim()" class="h-screen w-full border-0" allowfullscreen></iframe>
    <!-- SECURITY: homeContent is an admin-only setting. -->
    <div v-else v-html="homeContent"></div>
  </div>

  <div v-else-if="compactHomeEnabled" data-testid="compact-home" class="compact-home flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white">
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3"><img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-9 w-9 shrink-0 rounded-lg object-contain" /><span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span></div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="flex h-10 w-10 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800" :title="t('home.viewDocs')"><Icon name="book" size="md" /></a>
          <button class="flex h-10 w-10 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800" :title="isDark ? t('home.switchToLight') : t('home.switchToDark')" @click="toggleTheme"><Icon v-if="isDark" name="sun" size="md" /><Icon v-else name="moon" size="md" /></button>
          <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="inline-flex min-h-10 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900">{{ isAuthenticated ? t('home.dashboard') : t('home.login') }}</router-link>
          <router-link to="/usage-guide" class="inline-flex h-10 items-center gap-1.5 rounded-lg px-3 text-sm font-medium text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"><Icon name="book" size="md" /><span class="hidden sm:inline">{{ t('nav.guide') }}</span></router-link>
        </div>
      </nav>
    </header>
    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6"><div class="min-w-0 max-w-2xl text-center"><img :src="siteLogo || '/logo.svg'" alt="Logo" class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain" /><h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1><p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p><router-link :to="isAuthenticated ? dashboardPath : '/login'" class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700">{{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}</router-link></div></main>
    <footer class="border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 dark:border-dark-800 dark:text-dark-400">&copy; {{ currentYear }} {{ siteName }}</footer>
  </div>

  <div v-else class="home-page relative flex min-h-screen flex-col overflow-hidden bg-[#fbfaf8] text-[#11100e] dark:bg-[#111412] dark:text-white">
    <div class="home-grid pointer-events-none absolute inset-0" aria-hidden="true"></div>

    <header class="home-header relative z-20 border-b border-black/10 bg-[#fbfaf8]/90 px-5 backdrop-blur-md dark:border-white/10 dark:bg-[#111412]/90 sm:px-8">
      <nav class="mx-auto flex h-[76px] max-w-[1200px] items-center justify-between gap-5">
        <router-link to="/home" class="flex min-w-0 items-center gap-3" aria-label="返回首页"><span class="home-logo h-10 w-10 shrink-0 overflow-hidden rounded-xl border border-black/10 bg-white dark:border-white/10 dark:bg-white/10"><img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" /></span><span class="truncate text-base font-semibold tracking-tight">{{ siteName }}</span></router-link>
        <div class="flex shrink-0 items-center gap-1 sm:gap-2">
          <router-link to="/model-plaza" class="home-nav-link hidden sm:inline-flex">{{ t('nav.modelPlaza') }}</router-link>
          <router-link to="/usage-guide" class="home-nav-link hidden sm:inline-flex">{{ t('nav.guide') }}</router-link>
          <router-link to="/monitor" class="home-nav-link hidden md:inline-flex">{{ t('nav.channelStatus') }}</router-link>
          <LocaleSwitcher />
          <button class="home-icon-button" :title="isDark ? t('home.switchToLight') : t('home.switchToDark')" @click="toggleTheme"><Icon v-if="isDark" name="sun" size="md" /><Icon v-else name="moon" size="md" /></button>
          <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="home-login-button">{{ isAuthenticated ? t('home.dashboard') : t('home.login') }}</router-link>
        </div>
      </nav>
    </header>

    <main class="relative z-10 flex-1">
      <section class="home-hero mx-auto grid max-w-[1200px] items-center gap-12 px-5 pb-20 pt-16 sm:px-8 sm:pt-24 lg:grid-cols-[0.82fr_1.18fr] lg:gap-16 lg:pb-24">
        <div class="home-hero-copy min-w-0"><p class="mb-6 text-xs font-semibold uppercase tracking-[0.2em] text-[#168f85]">AI API INFRASTRUCTURE</p><h1 class="max-w-[560px] text-5xl font-semibold leading-[1.08] tracking-[-0.045em] text-[#11100e] dark:text-white sm:text-6xl lg:text-[4.45rem]">模型调用，<br />一站式搞定。</h1><p class="mt-7 max-w-[450px] text-base leading-7 text-[#6f746f] dark:text-[#aeb8b1] sm:text-lg">统一管理模型调用、API 密钥与用量。</p><router-link :to="isAuthenticated ? dashboardPath : '/login'" class="home-primary-button mt-9 inline-flex items-center gap-3">{{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}<Icon name="arrowRight" size="sm" :stroke-width="2" /></router-link></div>

        <div class="terminal-container home-dashboard-preview min-w-0">
          <div class="preview-topbar flex items-center justify-between gap-3 border-b border-black/[0.08] px-5 py-4 dark:border-white/10"><div class="flex min-w-0 items-center gap-2.5"><span class="preview-mark"><img :src="siteLogo || '/logo.svg'" alt="" class="h-full w-full object-contain" /></span><span class="truncate text-sm font-semibold">{{ siteName }}</span></div><div class="flex items-center gap-3"><span class="hidden items-center gap-2 text-xs text-[#66716b] sm:flex dark:text-[#b7c1bb]"><i class="status-dot"></i>服务状态&nbsp; 正常</span><span class="preview-select">最近 7 天 <Icon name="chevronDown" size="xs" /></span></div></div>
          <div class="preview-layout grid grid-cols-[132px_minmax(0,1fr)]"><aside class="preview-sidebar hidden border-r border-black/[0.08] px-3 py-5 sm:block dark:border-white/10"><div class="preview-side-item is-active"><Icon name="home" size="sm" />概览</div><div class="preview-side-item"><Icon name="server" size="sm" />模型路由</div><div class="preview-side-item"><Icon name="key" size="sm" />API 密钥</div><div class="preview-side-item"><Icon name="chartBar" size="sm" />用量统计</div><div class="preview-side-item"><Icon name="cog" size="sm" />设置</div></aside><div class="min-w-0 p-4 sm:p-6"><div class="grid gap-3 sm:grid-cols-3"><div class="preview-stat"><div class="preview-stat-title"><Icon name="server" size="sm" />模型路由</div><strong>36</strong><span>已配置模型 <Icon name="chevronRight" size="xs" /></span></div><div class="preview-stat"><div class="preview-stat-title"><Icon name="lock" size="sm" />API 密钥</div><strong>12</strong><span>已创建密钥 <Icon name="chevronRight" size="xs" /></span></div><div class="preview-stat"><div class="preview-stat-title"><Icon name="trendingUp" size="sm" />用量趋势</div><strong>128,520</strong><span>总调用次数 <Icon name="chevronRight" size="xs" /></span></div></div><div class="preview-chart mt-4"><div class="mb-4 flex items-center justify-between"><span class="text-sm font-semibold">用量趋势</span><Icon name="infoCircle" size="sm" class="text-[#9ba59e]" /></div><div class="chart-area" aria-hidden="true"><div class="chart-grid-lines"></div><svg viewBox="0 0 640 170" preserveAspectRatio="none"><polyline points="8,133 100,105 188,118 278,58 367,112 452,69 540,84 632,24" fill="none" stroke="currentColor" stroke-width="3" vector-effect="non-scaling-stroke" /><circle cx="8" cy="133" r="4" /><circle cx="100" cy="105" r="4" /><circle cx="188" cy="118" r="4" /><circle cx="278" cy="58" r="4" /><circle cx="367" cy="112" r="4" /><circle cx="452" cy="69" r="4" /><circle cx="540" cy="84" r="4" /><circle cx="632" cy="24" r="4" /></svg><div class="chart-labels"><span>05-11</span><span>05-12</span><span>05-13</span><span>05-14</span><span>05-15</span><span>05-16</span><span>05-17</span></div></div></div></div></div>
        </div>
      </section>

      <section class="home-features border-t border-black/[0.08] bg-white/55 dark:border-white/10 dark:bg-white/[0.025]"><div class="mx-auto grid max-w-[1200px] grid-cols-1 px-5 sm:px-8 md:grid-cols-3"><div class="home-feature"><span class="feature-icon"><Icon name="grid" size="lg" /></span><div><h2>模型广场</h2><p>聚合多种优质模型，支持快速接入与灵活路由。</p></div></div><div class="home-feature"><span class="feature-icon"><Icon name="lock" size="lg" /></span><div><h2>密钥管理</h2><p>集中管理 API 密钥，支持创建、轮换与权限控制。</p></div></div><div class="home-feature"><span class="feature-icon"><Icon name="trendingUp" size="lg" /></span><div><h2>用量可视化</h2><p>实时查看调用趋势与用量统计，助力成本优化。</p></div></div></div></section>
    </main>

    <footer class="relative z-10 flex flex-col items-center justify-between gap-3 border-t border-black/[0.08] px-5 py-6 text-xs text-[#7a817c] dark:border-white/10 dark:text-[#aeb8b1] sm:flex-row sm:px-8"><span>&copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}</span><div class="flex items-center gap-4"><a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="hover:text-[#168f85]">{{ t('home.docs') }}</a><a :href="githubUrl" target="_blank" rel="noopener noreferrer" class="hover:text-[#168f85]">GitHub</a></div></footer>
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
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Key 智中转')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || '一个密钥，统一接入多模型 API')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const isHomeContentUrl = computed(() => { const content = homeContent.value.trim(); return content.startsWith('http://') || content.startsWith('https://') })
const isDark = ref(document.documentElement.classList.contains('dark'))
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const currentYear = computed(() => new Date().getFullYear())

function toggleTheme() { isDark.value = !isDark.value; document.documentElement.classList.toggle('dark', isDark.value); localStorage.setItem('theme', isDark.value ? 'dark' : 'light') }
function initTheme() { const savedTheme = localStorage.getItem('theme'); if (savedTheme === 'dark' || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)) { isDark.value = true; document.documentElement.classList.add('dark') } }
onMounted(() => { initTheme(); authStore.checkAuth(); if (!appStore.publicSettingsLoaded) appStore.fetchPublicSettings() })
</script>

<style scoped>
.home-page { isolation: isolate; }
.home-grid { background-image: linear-gradient(rgba(17, 16, 14, .045) 1px, transparent 1px), linear-gradient(90deg, rgba(17, 16, 14, .045) 1px, transparent 1px); background-size: 34px 34px; }
.dark .home-grid { background-image: linear-gradient(rgba(255, 255, 255, .045) 1px, transparent 1px), linear-gradient(90deg, rgba(255, 255, 255, .045) 1px, transparent 1px); }
.home-nav-link { align-items: center; min-height: 40px; padding: 0 10px; border-radius: 8px; color: #646b66; font-size: .875rem; font-weight: 500; transition: color .18s ease, background-color .18s ease; }
.home-nav-link:hover { color: #11100e; background: rgba(17, 16, 14, .05); }
.dark .home-nav-link { color: #aeb8b1; }
.dark .home-nav-link:hover { color: #fff; background: rgba(255, 255, 255, .08); }
.home-icon-button { display: inline-flex; width: 40px; height: 40px; align-items: center; justify-content: center; border-radius: 8px; color: #68706a; transition: color .18s ease, background-color .18s ease; }
.home-icon-button:hover { color: #11100e; background: rgba(17, 16, 14, .05); }
.dark .home-icon-button { color: #b4beb7; }
.dark .home-icon-button:hover { color: #fff; background: rgba(255, 255, 255, .08); }
.home-login-button { display: inline-flex; min-height: 40px; align-items: center; justify-content: center; padding: 0 17px; border: 1px solid rgba(17, 16, 14, .14); border-radius: 9px; background: #fff; color: #333934; font-size: .875rem; font-weight: 600; box-shadow: 0 3px 12px rgba(17, 16, 14, .08); transition: border-color .18s ease, background-color .18s ease, transform .18s ease; }
.home-login-button:hover { border-color: rgba(22, 143, 133, .42); background: #f9fffd; transform: translateY(-1px); }
.dark .home-login-button { border-color: rgba(255, 255, 255, .16); background: #1c231f; color: #f6faf7; }
.dark .home-login-button:hover { background: #26312b; }
.home-primary-button { min-height: 48px; padding: 0 22px; border-radius: 9px; background: #168f85; color: #fff; font-size: .9375rem; font-weight: 600; box-shadow: 0 8px 18px rgba(22, 143, 133, .18); transition: background-color .18s ease, transform .18s ease, box-shadow .18s ease; }
.home-primary-button:hover { background: #11766e; transform: translateY(-2px); box-shadow: 0 10px 22px rgba(22, 143, 133, .24); }
.home-dashboard-preview { overflow: hidden; border: 1px solid rgba(17, 16, 14, .1); border-radius: 16px; background: rgba(255, 255, 255, .88); box-shadow: 0 22px 50px rgba(17, 16, 14, .1), 0 2px 8px rgba(17, 16, 14, .04); }
.dark .home-dashboard-preview { border-color: rgba(255, 255, 255, .12); background: rgba(28, 35, 31, .94); box-shadow: 0 24px 55px rgba(0, 0, 0, .28); }
.preview-mark { display: inline-flex; width: 28px; height: 28px; align-items: center; justify-content: center; overflow: hidden; border-radius: 7px; border: 1px solid rgba(17, 16, 14, .1); background: #f4faf8; }
.preview-select { display: inline-flex; min-height: 32px; align-items: center; gap: 8px; padding: 0 9px; border: 1px solid rgba(17, 16, 14, .1); border-radius: 7px; color: #66716b; font-size: .7rem; }
.dark .preview-select { border-color: rgba(255, 255, 255, .13); color: #b7c1bb; }
.preview-sidebar { color: #77817a; font-size: .7rem; }
.preview-side-item { display: flex; align-items: center; gap: 8px; min-height: 34px; margin-bottom: 5px; padding: 0 8px; border-radius: 7px; white-space: nowrap; }
.preview-side-item.is-active { background: #edf8f4; color: #168f85; font-weight: 600; }
.dark .preview-side-item.is-active { background: rgba(22, 143, 133, .16); color: #72d1c2; }
.preview-stat { min-width: 0; padding: 15px; border: 1px solid rgba(17, 16, 14, .08); border-radius: 10px; background: rgba(255, 255, 255, .8); }
.dark .preview-stat { border-color: rgba(255, 255, 255, .1); background: rgba(255, 255, 255, .025); }
.preview-stat-title { display: flex; align-items: center; gap: 7px; color: #168f85; font-size: .72rem; font-weight: 600; }
.preview-stat strong { display: block; margin-top: 12px; color: #171915; font-size: 1.55rem; line-height: 1; letter-spacing: -.03em; }
.dark .preview-stat strong { color: #f4faf5; }
.preview-stat > span { display: flex; align-items: center; justify-content: space-between; gap: 5px; margin-top: 9px; color: #929b94; font-size: .65rem; }
.preview-chart { min-height: 216px; padding: 17px; border: 1px solid rgba(17, 16, 14, .08); border-radius: 10px; background: rgba(255, 255, 255, .82); }
.dark .preview-chart { border-color: rgba(255, 255, 255, .1); background: rgba(255, 255, 255, .025); }
.chart-area { position: relative; height: 155px; color: #168f85; }
.chart-grid-lines { position: absolute; inset: 0 0 24px; background: repeating-linear-gradient(to bottom, rgba(17, 16, 14, .07) 0, rgba(17, 16, 14, .07) 1px, transparent 1px, transparent 25%); }
.dark .chart-grid-lines { background: repeating-linear-gradient(to bottom, rgba(255, 255, 255, .09) 0, rgba(255, 255, 255, .09) 1px, transparent 1px, transparent 25%); }
.chart-area svg { position: absolute; inset: 0 0 25px; width: 100%; height: calc(100% - 25px); overflow: visible; }
.chart-area circle { fill: #168f85; }
.chart-labels { position: absolute; right: 0; bottom: 0; left: 0; display: flex; justify-content: space-between; color: #929b94; font-size: .58rem; }
.home-features { position: relative; }
.home-feature { display: flex; align-items: flex-start; gap: 18px; min-height: 150px; padding: 37px 28px; }
.home-feature + .home-feature { border-left: 1px solid rgba(17, 16, 14, .1); }
.dark .home-feature + .home-feature { border-color: rgba(255, 255, 255, .1); }
.feature-icon { display: inline-flex; width: 48px; height: 48px; flex: 0 0 auto; align-items: center; justify-content: center; border-radius: 11px; background: #eaf7f3; color: #168f85; }
.dark .feature-icon { background: rgba(22, 143, 133, .16); color: #72d1c2; }
.home-feature h2 { margin-top: 2px; color: #171915; font-size: 1.1rem; font-weight: 650; }
.dark .home-feature h2 { color: #f4faf5; }
.home-feature p { max-width: 235px; margin-top: 8px; color: #737b75; font-size: .85rem; line-height: 1.7; }
.dark .home-feature p { color: #aeb8b1; }
.compact-home { background: #f7f8f6; }
@media (max-width: 767px) {
  .home-header nav { height: 68px; }
  .home-hero { gap: 38px; }
  .home-hero-copy { text-align: center; }
  .home-hero-copy h1, .home-hero-copy p { margin-right: auto; margin-left: auto; }
  .home-feature { min-height: 0; padding: 26px 0; }
  .home-feature + .home-feature { border-top: 1px solid rgba(17, 16, 14, .1); border-left: 0; }
  .dark .home-feature + .home-feature { border-color: rgba(255, 255, 255, .1); }
}
</style>
