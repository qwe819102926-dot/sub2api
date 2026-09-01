/**
 * Customer service configuration helpers.
 *
 * The admin configures customer support via the existing `contact_info` public
 * setting (Site Settings -> 客服联系方式). To keep things backwards compatible,
 * the field accepts **either** a plain support string (rendered verbatim) **or** a
 * JSON object describing a customer-service card with QR codes, e.g.:
 *
 * ```json
 * {
 *   "title": "联系客服",
 *   "subtitle": "QQ与群二维码",
 *   "items": [
 *     { "label": "客服 QQ", "value": "819102926", "qrcode": "https://.../qq.png" },
 *     { "label": "QQ群", "value": "2168062345", "qrcode": "https://.../group.png" }
 *   ]
 * }
 * ```
 *
 * A single QR entry can also be flattened at the top level:
 * `{ "label": "...", "value": "...", "qrcode": "https://..." }`.
 */

export interface CustomerServiceItem {
  label?: string
  value?: string
  qrcode?: string
}

export interface CustomerServiceConfig {
  title?: string
  subtitle?: string
  items?: CustomerServiceItem[]
  // Flattened single-entry support
  label?: string
  value?: string
  qrcode?: string
}

const isNonEmptyString = (v: unknown): v is string =>
  typeof v === 'string' && v.trim().length > 0

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function buildItem(value: unknown): CustomerServiceItem | null {
  if (!isRecord(value)) return null
  const item: CustomerServiceItem = {}
  if (isNonEmptyString(value.label)) item.label = value.label.trim()
  if (isNonEmptyString(value.value)) item.value = value.value.trim()
  if (isNonEmptyString(value.qrcode)) item.qrcode = value.qrcode.trim()
  return Object.keys(item).length ? item : null
}

/**
 * Parse a `contact_info` value into a customer-service config.
 * Returns `null` when the value is empty, not valid JSON, or does not describe a
 * customer-service card (so callers can fall back to plain-text rendering).
 */
export function parseCustomerServiceConfig(raw: string | null | undefined): CustomerServiceConfig | null {
  if (!raw) return null
  const trimmed = raw.trim()
  if (!trimmed || !trimmed.startsWith('{')) return null

  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    return null
  }
  if (!isRecord(parsed)) return null

  const items = Array.isArray(parsed.items) ? parsed.items.map(buildItem).filter((i): i is CustomerServiceItem => i !== null) : []
  const flattenQR = isNonEmptyString(parsed.qrcode) && items.length === 0
  if (items.length === 0 && !flattenQR) return null

  return {
    title: isNonEmptyString(parsed.title) ? parsed.title.trim() : undefined,
    subtitle: isNonEmptyString(parsed.subtitle) ? parsed.subtitle.trim() : undefined,
    items,
    label: isNonEmptyString(parsed.label) ? parsed.label.trim() : undefined,
    value: isNonEmptyString(parsed.value) ? parsed.value.trim() : undefined,
    qrcode: isNonEmptyString(parsed.qrcode) ? parsed.qrcode.trim() : undefined,
  }
}

/**
 * True when `contact_info` is a structured customer-service config (QR card).
 */
export function isCustomerServiceConfig(raw: string | null | undefined): boolean {
  return parseCustomerServiceConfig(raw) !== null
}

/**
 * Flatten a config into the list of customer-service entries to render.
 * Falls back to the top-level single entry when no `items` are provided.
 */
export function getCustomerServiceItems(config: CustomerServiceConfig | null): CustomerServiceItem[] {
  if (!config) return []
  const items = [...(Array.isArray(config.items) ? config.items : [])]
  if (isNonEmptyString(config.qrcode) && !items.some((item) => item.qrcode === config.qrcode)) {
    items.push({
      label: config.label,
      value: config.value,
      qrcode: config.qrcode,
    })
  }
  return items.filter((item) => isNonEmptyString(item.qrcode) || isNonEmptyString(item.value))
}

/**
 * Human-friendly contact text used where `contact_info` is rendered inline.
 * Returns the configured title (or a summary) when the value is a structured
 * config, otherwise the raw value verbatim.
 */
export function getContactDisplayText(raw: string | null | undefined): string {
  const config = parseCustomerServiceConfig(raw)
  if (!config) return raw?.trim() ?? ''
  if (isNonEmptyString(config.title)) return config.title
  const items = getCustomerServiceItems(config)
  if (items.length) return items[0].label ?? items[0].value ?? ''
  return ''
}
