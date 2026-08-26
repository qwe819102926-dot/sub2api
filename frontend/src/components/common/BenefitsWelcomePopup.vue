<template>
  <Teleport to="body">
    <Transition name="benefits-fade">
      <div v-if="show && hasActive" class="benefits-overlay" role="dialog" aria-modal="true" aria-labelledby="benefits-title">
        <section class="benefits-dialog" @click.stop>
          <header class="benefits-header">
            <h2 id="benefits-title">平台活动与福利</h2>
            <button type="button" class="benefits-close" aria-label="关闭" @click="close()"><Icon name="x" size="md" /></button>
          </header>

          <div class="benefits-body">
            <div class="benefits-banner">
              <div class="benefits-banner-copy">
                <p class="benefits-eyebrow"><Icon name="sparkles" size="sm" /> key智中转活动速览</p>
                <h1>欢迎回来，别错过本期福利</h1>
                <p>充值、使用和分享都能获得额外回馈。现在就来看看限时开放的活动。</p>
              </div>
              <div class="benefits-banner-art"><Icon name="gift" size="xl" /></div>
            </div>

            <div class="benefits-grid">
              <article v-for="activity in activities" :key="activity.id" class="benefit-card" :class="{ 'benefit-card-disabled': !activity.enabled }">
                <div class="benefit-card-top">
                  <div class="benefit-icon" :class="`benefit-icon-${activity.tone}`"><Icon :name="activity.icon" size="lg" /></div>
                  <span class="benefit-status" :class="{ 'benefit-status-ended': !activity.enabled }">{{ activity.enabled ? '进行中' : '已结束' }}</span>
                </div>
                <h3>{{ activity.title }}</h3>
                <p>{{ activity.description }}</p>
                <button type="button" class="benefit-action" :disabled="!activity.enabled" @click="go(activity.target)">{{ activity.action }} <Icon name="arrowRight" size="sm" /></button>
              </article>
            </div>

            <button type="button" class="benefit-affiliate" :class="{ 'benefit-card-disabled': !affiliateEnabled }" :disabled="!affiliateEnabled" @click="go('/affiliate')">
              <div class="benefit-icon benefit-icon-teal"><Icon name="users" size="lg" /></div>
              <div class="benefit-affiliate-copy"><div class="benefit-affiliate-title"><h3>邀请返利已开启</h3><span class="benefit-status" :class="{ 'benefit-status-ended': !affiliateEnabled }">{{ affiliateEnabled ? '已开启' : '已结束' }}</span></div><p>分享你的邀请链接，好友注册并充值后，你可以获得对应比例的返利额度。</p></div>
              <Icon name="arrowRight" size="md" class="benefit-affiliate-arrow" />
            </button>
            <p class="benefits-note">活动规则和最终到账金额以活动页面展示为准。</p>
          </div>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { paymentAPI } from '@/api/payment'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ close: []; active: [value: boolean] }>()
const router = useRouter(); const appStore = useAppStore(); const authStore = useAuthStore()
const rechargeBonusEnabled = ref(false); const consumptionEnabled = ref(false); const lotteryEnabled = ref(false)
const affiliateEnabled = computed(() => appStore.cachedPublicSettings?.affiliate_enabled === true)
const activities = computed(() => [
  { id: 'recharge', icon: 'gift' as const, tone: 'coral', title: '充值赠送', description: '充值时按活动规则额外到账赠送额度，到账后可直接用于 API 消费。', action: '去充值', target: '/purchase', enabled: rechargeBonusEnabled.value },
  { id: 'lottery', icon: 'sparkles' as const, tone: 'gold', title: '充值抽奖', description: '单次充值满 10 即可获得一次抽奖机会，奖品随机发放。', action: '查看奖励', target: '/dashboard?openLottery=1', enabled: lotteryEnabled.value },
  { id: 'consumption', icon: 'chartBar' as const, tone: 'teal', title: '消费奖励', description: '累计消费达到阶梯目标后，可领取对应的赠送额度。', action: '进入主页面', target: '/dashboard#consumption-reward', enabled: consumptionEnabled.value },
])
const hasActive = computed(() => rechargeBonusEnabled.value || consumptionEnabled.value || lotteryEnabled.value || affiliateEnabled.value)

async function loadActivityState() {
  const [rewards, lottery] = await Promise.allSettled([paymentAPI.getRewardCampaigns(), paymentAPI.getRechargeLottery()])
  if (rewards.status === 'fulfilled') {
    rechargeBonusEnabled.value = rewards.value.data.recharge_bonus.enabled
    consumptionEnabled.value = rewards.value.data.consumption_reward.enabled
  }
  if (lottery.status === 'fulfilled') lotteryEnabled.value = lottery.value.data.enabled
  emit('active', hasActive.value)
}
function close(redirectToDashboard = true) {
  emit('close')
  if (redirectToDashboard && router.currentRoute.value.path !== '/dashboard') void router.push('/dashboard')
}
function go(path: string) { close(false); void router.push(path) }
watch([() => props.show, hasActive], ([value, active]) => {
  document.body.classList.toggle('modal-open', value && active)
  if (value && authStore.isAuthenticated) loadActivityState().catch(() => emit('active', false))
}, { immediate: true })
watch(() => authStore.isAuthenticated, (value) => { if (value && props.show) loadActivityState().catch(() => emit('active', false)) })
watch(() => appStore.cachedPublicSettings?.affiliate_enabled, () => emit('active', hasActive.value))
onBeforeUnmount(() => document.body.classList.remove('modal-open'))
</script>

<style scoped>
.benefits-overlay { position: fixed; inset: 0; z-index: 130; display: flex; align-items: center; justify-content: center; overflow-y: auto; padding: 24px; background: rgba(15, 23, 42, .58); }
.benefits-dialog { width: min(1088px, 100%); max-height: min(92vh, 820px); overflow: hidden; border-radius: 18px; background: #fff; box-shadow: 0 24px 80px rgba(15, 23, 42, .28); }
.benefits-header { display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid #e5e7eb; padding: 22px 30px; }
.benefits-header h2 { margin: 0; color: #111827; font-size: 24px; font-weight: 750; }
.benefits-close { color: #94a3b8; transition: color .2s; } .benefits-close:hover { color: #334155; }
.benefits-body { overflow-y: auto; padding: 20px 30px 24px; }
.benefits-banner { position: relative; display: flex; min-height: 206px; align-items: center; justify-content: space-between; overflow: hidden; border: 1px solid #f2dfbc; border-radius: 16px; background: linear-gradient(115deg, #fff7e8 0%, #fffaf2 58%, #f7efe1 100%); padding: 32px 30px; color: #1f2937; }
.benefits-banner::after { position: absolute; right: -70px; top: -160px; width: 430px; height: 430px; border: 1px solid rgba(222, 183, 113, .28); border-radius: 50%; box-shadow: -38px 0 0 rgba(241, 218, 173, .22), -76px 0 0 rgba(247, 231, 198, .35); content: ''; }
.benefits-banner-copy { position: relative; z-index: 1; max-width: 690px; } .benefits-eyebrow { display: flex; align-items: center; gap: 8px; margin: 0 0 18px; color: #8b6a3e; font-size: 14px; font-weight: 700; letter-spacing: .04em; }
.benefits-banner h1 { margin: 0 0 14px; color: #1f2937; font-size: clamp(28px, 3.4vw, 42px); line-height: 1.15; letter-spacing: 0; } .benefits-banner-copy > p:last-child { margin: 0; color: #8b95a4; font-size: 16px; line-height: 1.7; }
.benefits-banner-art { position: relative; z-index: 1; display: grid; height: 82px; width: 82px; place-items: center; margin-right: 18px; color: #b28750; }
.benefits-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; margin-top: 24px; }
.benefit-card { min-height: 178px; display: flex; flex-direction: column; border: 1px solid #e5e7eb; border-radius: 16px; background: #fff; padding: 20px; transition: border-color .2s, box-shadow .2s; } .benefit-card:hover { border-color: #cbd5e1; box-shadow: 0 10px 24px rgba(15, 23, 42, .06); }
.benefit-card-disabled { filter: grayscale(.7); opacity: .58; } .benefit-card-top { display: flex; align-items: flex-start; justify-content: space-between; }
.benefit-icon { display: grid; height: 50px; width: 50px; place-items: center; border-radius: 15px; } .benefit-icon-coral { color: #f45d4c; background: #fff0ed; } .benefit-icon-gold { color: #c88308; background: #fff7dc; } .benefit-icon-teal { color: #099b8f; background: #e2f7f4; }
.benefit-status { border-radius: 999px; background: #e5f7f2; color: #0b927d; padding: 5px 10px; font-size: 12px; font-weight: 700; white-space: nowrap; } .benefit-status-ended { background: #f1f5f9; color: #94a3b8; }
.benefit-card h3 { margin: 14px 0 5px; color: #172033; font-size: 17px; font-weight: 750; } .benefit-card p { margin: 0; color: #64748b; font-size: 14px; line-height: 1.65; }
.benefit-action { display: inline-flex; align-items: center; gap: 7px; margin-top: auto; padding-top: 14px; color: #182235; font-size: 14px; font-weight: 700; } .benefit-action:hover:not(:disabled) { color: #ef634f; } .benefit-action:disabled { cursor: not-allowed; color: #a8b1bd; }
.benefit-affiliate { display: flex; width: 100%; align-items: center; gap: 16px; margin-top: 20px; border: 1px solid #dfe7e5; border-radius: 17px; background: #f8fbfa; padding: 18px 20px; text-align: left; transition: border-color .2s; } .benefit-affiliate:hover:not(:disabled) { border-color: #91d7cb; }
.benefit-affiliate-copy { min-width: 0; flex: 1; } .benefit-affiliate-title { display: flex; align-items: center; gap: 10px; } .benefit-affiliate-title h3 { margin: 0; color: #172033; font-size: 17px; } .benefit-affiliate-copy p { margin: 6px 0 0; color: #64748b; font-size: 14px; } .benefit-affiliate-arrow { color: #94a3b8; }
.benefits-note { margin: 20px 0 0; color: #9aa7b7; text-align: center; font-size: 13px; }
.benefits-fade-enter-active, .benefits-fade-leave-active { transition: opacity .2s ease; } .benefits-fade-enter-from, .benefits-fade-leave-to { opacity: 0; } .benefits-fade-enter-from .benefits-dialog { transform: translateY(8px) scale(.98); }
@media (max-width: 760px) { .benefits-overlay { align-items: flex-start; padding: 10px; } .benefits-dialog { max-height: calc(100vh - 20px); } .benefits-header, .benefits-body { padding-left: 18px; padding-right: 18px; } .benefits-grid { grid-template-columns: 1fr; } .benefits-banner { min-height: 250px; padding: 26px 20px; } .benefits-banner-art { position: absolute; right: 14px; top: 20px; transform: scale(.7) rotate(4deg); } .benefits-banner-copy { padding-top: 26px; } }
</style>
