import { buildGatewayUrl } from './client'

export interface ImageGenerationRequest {
  model: string
  prompt: string
  size?: string
  quality?: string
  n?: number
  output_format?: string
  background?: string
  output_compression?: number
}

export interface GeneratedImage {
  url: string
  mimeType: string
  index: number
}

export interface ImageGenerationModel {
  id: string
}

export interface ImageGenerationModelsResponse {
  object: string
  data: ImageGenerationModel[]
}

async function parseError(response: Response): Promise<Error> {
  try {
    const body = await response.json()
    const message = body?.error?.message || body?.message || response.statusText
    return new Error(message)
  } catch {
    return new Error(response.statusText || `HTTP ${response.status}`)
  }
}

function authHeaders(apiKey: string): HeadersInit {
  return { Authorization: `Bearer ${apiKey}` }
}

export function isImageGenerationModel(model: string, platform?: string): boolean {
  const id = model.trim().toLowerCase()
  if (!id) return false
  if (platform === 'openai') return id.startsWith('gpt-image-') || id.startsWith('dall-e-')
  if (platform === 'grok') return id.startsWith('grok-imagine') && !id.includes('video')
  return (
    id.startsWith('gpt-image-') ||
    id.startsWith('dall-e-') ||
    (id.startsWith('grok-imagine') && !id.includes('video'))
  )
}

export async function listImageGenerationModels(apiKey: string, platform?: string): Promise<string[]> {
  const response = await fetch(buildGatewayUrl('/v1/models'), {
    headers: { ...authHeaders(apiKey), Accept: 'application/json' },
  })
  if (!response.ok) throw await parseError(response)

  const body = await response.json() as ImageGenerationModelsResponse
  const seen = new Set<string>()
  return (Array.isArray(body?.data) ? body.data : [])
    .map(item => String(item?.id || '').trim())
    .filter(model => {
      if (!isImageGenerationModel(model, platform) || seen.has(model)) return false
      seen.add(model)
      return true
    })
}

function normalizeImages(body: any): GeneratedImage[] {
  const data = Array.isArray(body?.data) ? body.data : []
  return data
    .map((item: any, index: number) => {
      if (typeof item?.b64_json === 'string' && item.b64_json) {
        const mimeType = item.mime_type || 'image/png'
        return { url: `data:${mimeType};base64,${item.b64_json}`, mimeType, index }
      }
      if (typeof item?.url === 'string' && item.url) {
        return { url: item.url, mimeType: item.mime_type || 'image/*', index }
      }
      return null
    })
    .filter(Boolean) as GeneratedImage[]
}

export async function generateImages(
  apiKey: string,
  payload: ImageGenerationRequest,
  sourceImage?: File | null,
): Promise<{ images: GeneratedImage[]; raw: unknown }> {
  const isEdit = Boolean(sourceImage)
  const endpoint = isEdit ? '/v1/images/edits' : '/v1/images/generations'
  let response: Response

  if (sourceImage) {
    const form = new FormData()
    form.append('model', payload.model)
    form.append('prompt', payload.prompt)
    if (payload.size) form.append('size', payload.size)
    if (payload.quality) form.append('quality', payload.quality)
    if (payload.n) form.append('n', String(payload.n))
    if (payload.output_format) form.append('output_format', payload.output_format)
    if (payload.background) form.append('background', payload.background)
    if (payload.output_compression !== undefined) form.append('output_compression', String(payload.output_compression))
    form.append('image', sourceImage)
    response = await fetch(buildGatewayUrl(endpoint), { method: 'POST', headers: authHeaders(apiKey), body: form })
  } else {
    response = await fetch(buildGatewayUrl(endpoint), {
      method: 'POST',
      headers: { ...authHeaders(apiKey), 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
  }

  if (!response.ok) throw await parseError(response)
  const raw = await response.json()
  return { images: normalizeImages(raw), raw }
}

export function downloadGeneratedImage(image: GeneratedImage, filename: string) {
  const link = document.createElement('a')
  link.href = image.url
  link.download = filename
  link.target = '_blank'
  link.rel = 'noopener'
  document.body.appendChild(link)
  link.click()
  link.remove()
}
