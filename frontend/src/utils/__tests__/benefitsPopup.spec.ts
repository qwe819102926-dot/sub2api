import { describe, expect, it } from 'vitest'
import { shouldShowBenefitsWelcomePopup } from '../benefitsPopup'

describe('shouldShowBenefitsWelcomePopup', () => {
  it('shows for a user who has just logged in', () => {
    expect(shouldShowBenefitsWelcomePopup(true, false, false)).toBe(true)
  })

  it('does not show when authentication is restored after a refresh', () => {
    expect(shouldShowBenefitsWelcomePopup(true, undefined, false)).toBe(false)
  })

  it('does not show for administrators', () => {
    expect(shouldShowBenefitsWelcomePopup(true, false, true)).toBe(false)
  })
})
