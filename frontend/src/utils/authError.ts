import { extractApiErrorCode } from './apiError'

export function registrationGuardMessageKey(error: unknown): string | undefined {
  if (extractApiErrorCode(error) === "REGISTRATION_IP_BLOCKED") return "auth.registrationIPBlocked"
  const code = extractApiErrorCode(error)
  if (code && ['REGISTRATION_BROWSER_REQUIRED', 'REGISTRATION_SESSION_INVALID', 'REGISTRATION_LIMITED'].includes(code)) {
    return `auth.errors.${code}`
  }
  return undefined
}

interface APIErrorLike {
  message?: string
  response?: {
    data?: {
      detail?: string
      message?: string
    }
  }
}

function extractErrorMessage(error: unknown): string {
  const err = (error || {}) as APIErrorLike
  return err.response?.data?.detail || err.response?.data?.message || err.message || ''
}

export function buildAuthErrorMessage(
  error: unknown,
  options: {
    fallback: string
  }
): string {
  const { fallback } = options
  const message = extractErrorMessage(error)
  return message || fallback
}
