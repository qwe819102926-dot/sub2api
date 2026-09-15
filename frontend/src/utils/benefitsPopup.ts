export function shouldShowBenefitsWelcomePopup(
  isAuthenticated: boolean,
  wasAuthenticated: boolean | undefined,
  isAdmin: boolean
): boolean {
  return isAuthenticated && wasAuthenticated === false && !isAdmin
}
