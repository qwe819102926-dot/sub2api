import type { ApiKey } from '@/types'

export function buildMagpieImportDeeplink(
  baseUrl: string,
  siteName: string,
  apiKey: Pick<ApiKey, 'id' | 'name' | 'key' | 'group'>
): string {
  const endpoint = new URL(baseUrl)
  if (
    !['https:', 'http:'].includes(endpoint.protocol) ||
    endpoint.username || endpoint.password || endpoint.search || endpoint.hash
  ) {
    throw new Error('Invalid Magpie API base URL')
  }

  const root = endpoint.origin + endpoint.pathname.replace(/\/+$/, '').replace(/\/v(?:1|1beta)$/, '')
  const site = siteName.trim() || 'sub2api'
  const keyName = apiKey.name.trim()
  const name = Array.from(keyName ? `${site} · ${keyName}` : site).slice(0, 80).join('')
  const host = endpoint.host.toLowerCase().replace(/[^a-z0-9]+/g, '-')
  const params = new URLSearchParams({
    name,
    id: `sub2api-${host}-${apiKey.id}`,
    key: apiKey.key
  })

  switch (apiKey.group?.platform) {
    case 'openai':
    case 'grok':
    case 'composite':
      params.set('responses', `${root}/v1`)
      break
    case 'kimi':
    case 'zhipu':
    case 'deepseek':
      params.set('chat', `${root}/v1`)
      break
    case 'gemini':
      params.set('gemini', `${root}/v1beta`)
      break
    case 'antigravity':
      params.set('anthropic', `${root}/antigravity`)
      params.set('gemini', `${root}/antigravity/v1beta`)
      break
    default:
      params.set('anthropic', root)
      break
  }

  return `magpie://import?${params}`
}
