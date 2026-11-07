import { describe, expect, it } from 'vitest'
import { groupSubscriptionPlans } from '../subscriptionCategories'
import type { SubscriptionPlan } from '@/types/payment'

describe('subscription categories', () => {
  it('sorts without mutating plans and puts unknown or missing platforms in Other', () => {
    const plans = [
      { id: 2, group_platform: 'openai', sort_order: 20 },
      { id: 3, group_platform: 'anthropic', sort_order: 0 },
      { id: 1, group_platform: 'openai', sort_order: 10 },
      { id: 4, group_platform: 'future-platform', sort_order: 0 },
      { id: 5, sort_order: 0 },
    ] as SubscriptionPlan[]
    const before = structuredClone(plans)
    const groups = groupSubscriptionPlans(plans)
    expect(groups.map(group => group.key)).toEqual(['gpt', 'claude', 'other'])
    expect(groups[0].plans.map(plan => plan.id)).toEqual([1, 2])
    expect(groups[2].plans.map(plan => plan.id)).toEqual([4, 5])
    expect(plans).toEqual(before)
  })
})
