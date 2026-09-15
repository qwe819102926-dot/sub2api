import { afterEach, describe, expect, it, vi } from 'vitest'
import { isImageGenerationModel, listImageGenerationModels } from '../imageGeneration'

describe('image generation models API', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('recognizes OpenAI and Grok image models without including video models', () => {
    expect(isImageGenerationModel('gpt-image-2', 'openai')).toBe(true)
    expect(isImageGenerationModel('dall-e-3', 'openai')).toBe(true)
    expect(isImageGenerationModel('gpt-5.6', 'openai')).toBe(false)
    expect(isImageGenerationModel('grok-imagine-image-2.0', 'grok')).toBe(true)
    expect(isImageGenerationModel('grok-imagine-video-1.5', 'grok')).toBe(false)
  })

  it('loads and deduplicates image models available to the selected key', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        object: 'list',
        data: [
          { id: 'gpt-image-2' },
          { id: 'gpt-5.6' },
          { id: 'gpt-image-2' },
          { id: 'gpt-image-2.5-flare' },
        ],
      }),
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(listImageGenerationModels('sk-test', 'openai')).resolves.toEqual([
      'gpt-image-2',
      'gpt-image-2.5-flare',
    ])
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining('/v1/models'),
      expect.objectContaining({ headers: { Accept: 'application/json', Authorization: 'Bearer sk-test' } }),
    )
  })
})
