import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { downloadGeneratedImage, generatedImageFilename, isImageGenerationModel, listImageGenerationModels } from '../imageGeneration'

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

function installObjectURLMethods() {
  if (typeof URL.createObjectURL !== 'function') {
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      writable: true,
      value: () => '',
    })
  }
  if (typeof URL.revokeObjectURL !== 'function') {
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      writable: true,
      value: () => undefined,
    })
  }
}

describe('generated image downloads', () => {
  beforeEach(() => {
    installObjectURLMethods()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
    document.body.replaceChildren()
  })

  it('names wildcard images from the selected output format', () => {
    expect(generatedImageFilename({ url: 'https://cdn.example.com/a', mimeType: 'image/*', index: 0 }, 'jpeg')).toBe('generated-1.jpg')
    expect(generatedImageFilename({ url: 'https://cdn.example.com/a', mimeType: 'image/png', index: 2 }, 'webp')).toBe('generated-3.png')
    expect(generatedImageFilename({ url: 'https://cdn.example.com/a', mimeType: 'image/webp', index: 0 }, 'png')).toBe('generated-1.webp')
  })

  it('downloads a remote image and revokes the blob URL after the browser starts saving it', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    const imageUrl = 'https://cdn.example.com/generated.png'
    const blob = new Blob(['image data'], { type: 'image/jpeg' })
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, blob: vi.fn().mockResolvedValue(blob) })
    const objectUrl = 'blob:https://app.example.test/generated'
    vi.spyOn(URL, 'createObjectURL').mockReturnValue(objectUrl)
    const revokeObjectUrl = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined)
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function () {
      expect(this.download).toBe('generated-1.jpg')
      expect(this.target).toBe('')
      expect(this.href).toBe(objectUrl)
    })
    vi.stubGlobal('fetch', fetchMock)

    await downloadGeneratedImage({ url: imageUrl, mimeType: 'image/*', index: 0 }, 'generated-1.png')

    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock).toHaveBeenCalledWith(imageUrl)
    expect(click).toHaveBeenCalledOnce()
    expect(revokeObjectUrl).not.toHaveBeenCalled()
    expect(document.body.querySelector('a')).toBeNull()
    await vi.advanceTimersByTimeAsync(999)
    expect(revokeObjectUrl).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1)
    expect(revokeObjectUrl).toHaveBeenCalledWith(objectUrl)
  })

  it('downloads data images synchronously without fetch', async () => {
    const fetchMock = vi.fn()
    const objectUrl = 'blob:https://app.example.test/data-image'
    vi.spyOn(URL, 'createObjectURL').mockReturnValue(objectUrl)
    vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined)
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function () {
      expect(fetchMock).not.toHaveBeenCalled()
      expect(this.download).toBe('generated-1.png')
      expect(this.href).toBe(objectUrl)
    })
    vi.stubGlobal('fetch', fetchMock)

    const pending = downloadGeneratedImage(
      { url: 'data:image/png;base64,aW1hZ2UgZGF0YQ==', mimeType: 'image/png', index: 0 },
      'generated-1.png',
    )

    expect(click).toHaveBeenCalledOnce()
    await pending
    expect(fetchMock).not.toHaveBeenCalled()
    const blob = vi.mocked(URL.createObjectURL).mock.calls[0][0] as Blob
    expect(blob.type).toBe('image/png')
  })

  it('falls back to the same-origin download gateway when direct fetch is blocked', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    const imageUrl = 'https://cdn.example.com/generated.png'
    const blob = new Blob(['image data'], { type: 'image/png' })
    const fetchMock = vi.fn()
      .mockRejectedValueOnce(new TypeError('Failed to fetch'))
      .mockResolvedValueOnce({ ok: true, blob: vi.fn().mockResolvedValue(blob) })
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:https://app.example.test/fallback')
    vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined)
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
    vi.stubGlobal('fetch', fetchMock)

    await downloadGeneratedImage({ url: imageUrl, mimeType: 'image/*', index: 0 }, 'generated-1.png', 'sk-test')

    expect(fetchMock).toHaveBeenNthCalledWith(1, imageUrl)
    expect(fetchMock).toHaveBeenNthCalledWith(2, expect.stringContaining('/v1/images/download'), expect.objectContaining({
      method: 'POST',
      headers: expect.objectContaining({
        Authorization: 'Bearer sk-test',
        'Content-Type': 'application/json',
      }),
      body: JSON.stringify({ url: imageUrl }),
    }))
    expect(click).toHaveBeenCalledOnce()
  })
})
