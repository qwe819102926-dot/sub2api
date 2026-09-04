<template>
  <div class="auth-shell relative flex min-h-screen items-stretch justify-center overflow-x-hidden">
    <div class="auth-grid pointer-events-none absolute inset-0"></div>

    <div class="auth-frame relative z-10 grid min-h-screen w-full overflow-hidden lg:grid-cols-[1.08fr_0.92fr]">
      <section class="auth-brand-panel relative hidden min-h-screen flex-col justify-between overflow-hidden p-10 lg:flex xl:p-14">
        <div class="auth-brand-pattern pointer-events-none absolute inset-0"></div>
        <div class="relative z-10">
          <div class="flex items-center gap-3">
            <div class="flex h-11 w-11 items-center justify-center overflow-hidden rounded-xl border border-white/15 bg-white">
              <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
            </div>
            <span class="text-sm font-semibold tracking-tight text-white">{{ siteName }}</span>
          </div>

          <div class="mt-24 max-w-xl">
            <p class="mb-5 text-[11px] font-semibold uppercase tracking-[0.24em] text-[#ff806c]">AI API GATEWAY</p>
            <h1 class="auth-brand-copy text-5xl font-semibold leading-[1.02] tracking-tight text-white xl:text-6xl">
              模型调用、密钥管理，<br />一站式搞定。
            </h1>
            <p class="mt-7 max-w-md text-base leading-7 text-white/55">
              登录后即可管理模型调用、API 密钥和用量。
            </p>
          </div>

          <div class="auth-model-orbit" aria-hidden="true">
            <span class="auth-orbit auth-orbit-one"></span>
            <span class="auth-orbit auth-orbit-two"></span>
            <span class="auth-orbit auth-orbit-three"></span>
            <span class="auth-model-node auth-model-openai">O</span>
            <span class="auth-model-node auth-model-claude">C</span>
            <span class="auth-model-node auth-model-gemini">✦</span>
            <span class="auth-model-core"></span>
          </div>
        </div>

        <div class="relative z-10 flex flex-wrap gap-2 border-t border-white/10 pt-5 text-[11px] font-medium uppercase tracking-[0.14em] text-white/45">
          <span>Unified access</span>
          <span class="text-white/20">/</span>
          <span>Usage insights</span>
          <span class="text-white/20">/</span>
          <span>Secure by default</span>
        </div>
      </section>

      <section class="auth-form-panel flex min-h-screen items-start px-5 py-9 sm:px-10 sm:py-12 lg:px-14 lg:py-14">
        <div class="mx-auto w-full max-w-md">
          <div class="mb-8 lg:hidden">
            <div class="mb-4 flex h-11 w-11 items-center justify-center overflow-hidden rounded-xl border border-gray-900/10 bg-white dark:border-white/10 dark:bg-white/5">
              <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
            </div>
            <p class="text-sm font-semibold tracking-tight text-gray-950 dark:text-white">{{ siteName }}</p>
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
const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.auth-shell {
  background: #f7f6f2;
}

.auth-grid {
  background-image:
    linear-gradient(rgba(18, 18, 18, 0.035) 1px, transparent 1px),
    linear-gradient(90deg, rgba(18, 18, 18, 0.035) 1px, transparent 1px);
  background-size: 34px 34px;
}

.auth-frame {
  border: 0;
  border-radius: 0;
  box-shadow: none;
}

.auth-brand-panel {
  background: #111312;
}

.auth-brand-pattern {
  opacity: 0.32;
  background-image:
    linear-gradient(135deg, transparent 0 49%, rgba(255,255,255,0.08) 50%, transparent 51%),
    linear-gradient(rgba(255,255,255,0.035) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255,255,255,0.035) 1px, transparent 1px);
  background-size: 130px 130px, 34px 34px, 34px 34px;
}

.auth-model-orbit {
  position: absolute;
  right: -4.5rem;
  bottom: 5rem;
  width: 25rem;
  height: 25rem;
  transform: rotate(-18deg);
  opacity: 0.9;
}

.auth-orbit {
  position: absolute;
  inset: 0;
  border: 1px solid rgba(255,255,255,0.18);
  border-radius: 50%;
}

.auth-orbit-one { transform: scaleX(0.42) rotate(38deg); }
.auth-orbit-two { transform: scaleX(0.62) rotate(-42deg); }
.auth-orbit-three { transform: scaleX(0.8) rotate(72deg); }

.auth-model-node {
  position: absolute;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.6rem;
  height: 2.6rem;
  border: 1px solid rgba(255,255,255,0.24);
  border-radius: 50%;
  background: #1d201e;
  color: #fff;
  font-size: 1rem;
  font-weight: 600;
  box-shadow: 0 8px 22px rgba(0,0,0,0.28);
}

.auth-model-openai { top: 1.25rem; left: 10.5rem; color: #9ce3b1; }
.auth-model-claude { right: 1.25rem; top: 11rem; color: #ffb08f; }
.auth-model-gemini { bottom: 1.25rem; left: 8rem; color: #a8c5ff; }

.auth-model-core {
  position: absolute;
  inset: 9.8rem;
  border: 1px solid rgba(255,255,255,0.28);
  border-radius: 50%;
  background: rgba(255,255,255,0.04);
  box-shadow: 0 0 0 18px rgba(255,255,255,0.025), 0 0 0 36px rgba(255,255,255,0.018);
}

.auth-form-panel {
  background: #fff;
}

.auth-form-panel :deep(.btn-primary) {
  min-height: 3.5rem;
  border-radius: 14px;
  background: #151615;
  box-shadow: 0 8px 18px rgba(18, 18, 18, 0.14);
}

.auth-form-panel :deep(.btn-primary:hover) {
  background: #343634;
  transform: translateY(-1px);
}

.auth-form-panel :deep(.input) {
  min-height: 3.5rem;
  border-radius: 14px;
  border-color: rgba(18, 18, 18, 0.16);
  background: #fff;
  color: #151615;
  box-shadow: none;
}

.auth-form-panel :deep(.input:focus) {
  border-color: #ff705a;
  box-shadow: 0 0 0 3px rgba(255, 112, 90, 0.14);
}

.auth-form-panel :deep(.input-label) {
  color: #282a28;
  font-size: 0.78rem;
  font-weight: 600;
  letter-spacing: 0.01em;
}

.auth-form-panel :deep(.auth-title) {
  text-align: left;
}

.auth-form-panel :deep(.auth-title h2) {
  font-size: 2rem;
  letter-spacing: -0.03em;
}

.auth-form-panel :deep(.auth-title p) {
  margin-top: 0.6rem;
  color: #777b76;
}

.dark .auth-shell {
  background: #151715;
}

.dark .auth-grid {
  background-image:
    linear-gradient(rgba(255,255,255,0.035) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255,255,255,0.035) 1px, transparent 1px);
}

.dark .auth-frame {
  border-color: rgba(255,255,255,0.13);
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.42);
}

.dark .auth-form-panel {
  background: #20231f;
}

.dark .auth-form-panel :deep(.input) {
  border-color: rgba(255,255,255,0.15);
  background: #20231f;
  color: #f7f7f2;
}

.dark .auth-form-panel :deep(.input-label) {
  color: #e5e8e0;
}

@media (max-width: 639px) {
  .auth-form-panel :deep(.auth-title h2) {
    font-size: 1.75rem;
  }
}
</style>
