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
})
