import { computed, ref } from 'vue'
import { keysAPI } from '@/api/keys'
import type { ApiKey } from '@/types'

const keys = ref<ApiKey[]>([])
const loaded = ref(false)
const loading = ref(false)

function keyAllowsImage(key: ApiKey) {
  const platform = key.group?.platform
  return key.status === 'active' && key.group?.allow_image_generation === true && (platform === 'openai' || platform === 'grok')
}

async function refreshImageGenerationAccess(force = false) {
  if (loading.value || (loaded.value && !force)) return
  loading.value = true
  try {
    const result = await keysAPI.list(1, 100, { status: 'active' })
    keys.value = result.items || []
  } catch {
    keys.value = []
  } finally {
    loaded.value = true
    loading.value = false
  }
}

export function useImageGenerationAccess() {
  return { imageGenerationKeys: keys, canUseImageGeneration: computed(() => keys.value.some(keyAllowsImage)), loadingImageGenerationAccess: loading, refreshImageGenerationAccess, keyAllowsImage }
}
