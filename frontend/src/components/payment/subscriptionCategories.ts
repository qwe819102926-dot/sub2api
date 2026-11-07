import type { SubscriptionPlan } from '@/types/payment'

export type SubscriptionCategoryKey = 'gpt' | 'claude' | 'deepseek' | 'other'

// Display configuration only. Lower sortOrder appears first; access and billing
// remain controlled by the plan's existing business group.
export const SUBSCRIPTION_CATEGORIES = [
  { key: 'gpt', platform: 'openai', sortOrder: 10 },
  { key: 'claude', platform: 'anthropic', sortOrder: 20 },
  { key: 'deepseek', platform: 'deepseek', sortOrder: 30 },
  { key: 'other', platform: '', sortOrder: 40 },
] as const

export function subscriptionCategory(platform?: string): SubscriptionCategoryKey {
  return SUBSCRIPTION_CATEGORIES.find(category => category.platform === platform)?.key ?? 'other'
}

export function groupSubscriptionPlans(plans: SubscriptionPlan[]) {
  return [...SUBSCRIPTION_CATEGORIES]
    .sort((a, b) => a.sortOrder - b.sortOrder)
    .map(category => ({
      ...category,
      plans: plans.filter(plan => subscriptionCategory(plan.group_platform) === category.key)
        .sort((a, b) => a.sort_order - b.sort_order || a.id - b.id),
    }))
    .filter(category => category.plans.length > 0)
}
