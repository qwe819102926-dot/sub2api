import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const viewPath = resolve(dirname(fileURLToPath(import.meta.url)), '../ImageGenerationView.vue')
const viewSource = readFileSync(viewPath, 'utf8')

describe('ImageGenerationView data loading', () => {
  it('refreshes keys on entry and loads models for the selected key', () => {
    expect(viewSource).toContain('await refreshImageGenerationAccess(true)')
    expect(viewSource).toContain('listImageGenerationModels(key.key, key.group?.platform)')
    expect(viewSource).toContain('@change="loadAvailableModels"')
  })

  it('labels request sizes with their 1K, 2K and 4K billing tiers', () => {
    expect(viewSource).toContain('<option value="1024x1024">1K · 1024x1024</option>')
    expect(viewSource).toContain('<option value="2048x2048">2K · 2048x2048</option>')
    expect(viewSource).toContain('<option value="3840x2160">4K · 3840x2160')
    expect(viewSource).toContain('<option value="2160x3840">4K · 2160x3840')
  })
})
