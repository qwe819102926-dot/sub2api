<template>
  <AppLayout>
    <div class="mx-auto max-w-[1440px] space-y-6">
      <div class="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p class="mb-1 text-sm font-medium text-primary-600 dark:text-primary-400">{{ t('imageGeneration.eyebrow') }}</p>
          <h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-white">{{ t('imageGeneration.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('imageGeneration.description') }}</p>
        </div>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="loadingAccess || loadingModels" @click="refreshPage">
          <Icon name="refresh" size="sm" class="mr-1.5" :class="loadingAccess || loadingModels ? 'animate-spin' : ''" />{{ t('common.refresh') }}
        </button>
      </div>

      <div v-if="!loadingAccess && imageKeys.length === 0" class="rounded-xl border border-amber-200 bg-amber-50 px-5 py-4 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-100">
        {{ t('imageGeneration.noKeys') }}
      </div>

      <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(380px,0.88fr)]">
        <section class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900 md:p-7">
          <div class="mb-6 flex items-center gap-3 border-b border-gray-100 pb-5 dark:border-dark-700">
            <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300"><Icon name="sparkles" size="md" /></div>
            <div><h2 class="font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.formTitle') }}</h2><p class="text-sm text-gray-500 dark:text-gray-400">{{ t('imageGeneration.formHint') }}</p></div>
          </div>

          <div class="mb-6 grid grid-cols-2 rounded-xl bg-gray-100 p-1 dark:bg-dark-800">
            <button type="button" class="rounded-lg px-4 py-2.5 text-sm font-medium transition" :class="mode === 'generate' ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-600 dark:text-white' : 'text-gray-500 dark:text-gray-400'" @click="mode = 'generate'; clearSourceImage()">{{ t('imageGeneration.textToImage') }}</button>
            <button type="button" class="rounded-lg px-4 py-2.5 text-sm font-medium transition" :class="mode === 'edit' ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-600 dark:text-white' : 'text-gray-500 dark:text-gray-400'" @click="mode = 'edit'">{{ t('imageGeneration.imageToImage') }}</button>
          </div>

          <div class="grid gap-4 md:grid-cols-2">
            <label class="field"><span>{{ t('imageGeneration.apiKey') }}</span><select v-model="selectedKeyId" class="input" @change="loadAvailableModels"><option :value="0">{{ t('imageGeneration.selectKey') }}</option><option v-for="key in imageKeys" :key="key.id" :value="key.id">{{ key.name }} · {{ key.group?.name || key.group?.platform }}</option></select></label>
            <label class="field">
              <span>{{ t('imageGeneration.model') }}</span>
              <select v-model="form.model" class="input" :disabled="loadingModels || availableModels.length === 0">
                <option v-if="loadingModels" value="">{{ t('imageGeneration.loadingModels') }}</option>
                <option v-else-if="availableModels.length === 0" value="">{{ t('imageGeneration.noModels') }}</option>
                <option v-for="model in availableModels" :key="model" :value="model">{{ model }}</option>
              </select>
              <small v-if="modelLoadError" class="!text-left !text-amber-600 dark:!text-amber-400">{{ modelLoadError }}</small>
            </label>
          </div>

          <label class="field mt-5"><span>{{ t('imageGeneration.prompt') }}</span><textarea v-model="form.prompt" rows="7" class="input resize-y" :placeholder="t('imageGeneration.promptPlaceholder')" maxlength="4000" /><small>{{ form.prompt.length }} / 4000</small></label>

          <div v-if="mode === 'edit'" class="mt-5 rounded-xl border border-dashed border-gray-300 p-4 dark:border-dark-600">
            <div class="flex flex-wrap items-center justify-between gap-3"><div><p class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ t('imageGeneration.referenceImage') }}</p><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('imageGeneration.referenceHint') }}</p></div><label class="btn btn-secondary btn-sm cursor-pointer"><Icon name="upload" size="sm" class="mr-1.5" />{{ t('common.chooseFile') }}<input type="file" class="hidden" accept="image/png,image/jpeg,image/webp" @change="onSourceImage" /></label></div>
            <div v-if="sourceImage" class="relative mt-4 overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700"><img :src="sourcePreview" class="max-h-52 w-full object-contain bg-gray-50 dark:bg-dark-800" :alt="sourceImage.name" /><button type="button" class="absolute right-2 top-2 rounded-full bg-black/60 p-1.5 text-white" :title="t('common.remove')" @click="clearSourceImage"><Icon name="x" size="sm" /></button></div>
          </div>

          <div class="mt-5 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <label class="field">
              <span>{{ t('imageGeneration.size') }}</span>
              <select v-model="form.size" class="input">
                <option value="1024x1024">1K · 1024x1024</option>
                <option value="1536x1024">2K · 1536x1024 ({{ t('imageGeneration.landscape') }})</option>
                <option value="1024x1536">2K · 1024x1536 ({{ t('imageGeneration.portrait') }})</option>
                <option value="2048x2048">2K · 2048x2048</option>
                <option value="3840x2160">4K · 3840x2160 ({{ t('imageGeneration.landscape') }})</option>
                <option value="2160x3840">4K · 2160x3840 ({{ t('imageGeneration.portrait') }})</option>
                <option value="auto">{{ t('imageGeneration.auto') }} · {{ t('imageGeneration.autoSizeBillingHint') }}</option>
              </select>
            </label>
            <label class="field"><span>{{ t('imageGeneration.quality') }}</span><select v-model="form.quality" class="input"><option value="auto">{{ t('imageGeneration.auto') }}</option><option value="low">{{ t('imageGeneration.low') }}</option><option value="medium">{{ t('imageGeneration.medium') }}</option><option value="high">{{ t('imageGeneration.high') }}</option></select></label>
            <label class="field"><span>{{ t('imageGeneration.count') }}</span><select v-model.number="form.n" class="input"><option :value="1">1</option><option :value="2">2</option><option :value="3">3</option><option :value="4">4</option></select></label>
            <label class="field"><span>{{ t('imageGeneration.format') }}</span><select v-model="form.output_format" class="input"><option value="png">PNG</option><option value="jpeg">JPEG</option><option value="webp">WebP</option></select></label>
          </div>

          <div class="mt-7 flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 pt-5 dark:border-dark-700"><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('imageGeneration.billingHint') }}</p><button type="button" class="btn btn-primary min-w-36" :disabled="submitting || !canSubmit" @click="submit"><Icon :name="submitting ? 'refresh' : 'sparkles'" size="sm" class="mr-2" :class="submitting ? 'animate-spin' : ''" />{{ submitting ? t('imageGeneration.generating') : t('imageGeneration.generate') }}</button></div>
          <p v-if="errorMessage" class="mt-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ errorMessage }}</p>
        </section>

        <section class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900 md:p-7">
          <div class="mb-5 flex items-center justify-between border-b border-gray-100 pb-5 dark:border-dark-700"><div><h2 class="font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.resultTitle') }}</h2><p class="text-sm text-gray-500 dark:text-gray-400">{{ submitting ? t('imageGeneration.resultPending') : images.length ? t('imageGeneration.resultReady', { count: images.length }) : t('imageGeneration.resultEmpty') }}</p></div><button v-if="images.length" type="button" class="btn btn-secondary btn-sm" @click="clearResult"><Icon name="x" size="sm" class="mr-1.5" />{{ t('common.clear') }}</button></div>
          <p v-if="downloadError" class="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ downloadError }}</p><div v-if="images.length" class="grid gap-4 sm:grid-cols-2"><figure v-for="image in images" :key="image.index" class="group overflow-hidden rounded-xl border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-800"><img :src="image.url" class="aspect-square w-full object-contain" :alt="t('imageGeneration.imageAlt', { index: image.index + 1 })" /><figcaption class="flex items-center justify-between border-t border-gray-200 bg-white px-3 py-2 dark:border-dark-700 dark:bg-dark-900"><span class="text-xs text-gray-500">{{ image.mimeType }}</span><button type="button" class="text-primary-600 hover:text-primary-700 dark:text-primary-300" :title="t('imageGeneration.download')" @click="downloadImage(image)"><Icon name="download" size="sm" /></button></figcaption></figure></div>
          <div v-else class="flex min-h-[520px] flex-col items-center justify-center rounded-xl border border-dashed border-gray-200 bg-gray-50/60 px-6 text-center dark:border-dark-700 dark:bg-dark-800/40"><Icon name="sparkles" size="lg" class="mb-4 text-gray-300 dark:text-dark-500" /><p class="font-medium text-gray-700 dark:text-gray-200">{{ submitting ? t('imageGeneration.generating') : t('imageGeneration.resultEmpty') }}</p><p class="mt-2 max-w-xs text-sm text-gray-500 dark:text-gray-400">{{ t('imageGeneration.resultEmptyHint') }}</p></div>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { Icon } from '@/components/icons'
import { useImageGenerationAccess } from '@/composables/useImageGenerationAccess'
import { generateImages, downloadGeneratedImage, generatedImageFilename, listImageGenerationModels, type GeneratedImage } from '@/api/imageGeneration'

const { t } = useI18n()
const { imageGenerationKeys: imageKeys, loadingImageGenerationAccess: loadingAccess, refreshImageGenerationAccess } = useImageGenerationAccess()
const selectedKeyId = ref(0)
const mode = ref<'generate' | 'edit'>('generate')
const submitting = ref(false)
const loadingModels = ref(false)
const errorMessage = ref('')
const downloadError = ref('')
const modelLoadError = ref('')
const availableModels = ref<string[]>([])
const images = ref<GeneratedImage[]>([])
const sourceImage = ref<File | null>(null)
const sourcePreview = ref('')
const form = ref({ model: '', prompt: '', size: '1024x1024', quality: 'auto', n: 1, output_format: 'png' })
const selectedKey = computed(() => imageKeys.value.find(key => key.id === selectedKeyId.value))
const canSubmit = computed(() => Boolean(selectedKey.value?.key && availableModels.value.includes(form.value.model) && form.value.prompt.trim() && (mode.value === 'generate' || sourceImage.value)))
let modelRequestSeq = 0

onMounted(refreshPage)
onBeforeUnmount(() => { if (sourcePreview.value) URL.revokeObjectURL(sourcePreview.value) })

async function refreshPage() {
  await refreshImageGenerationAccess(true)
  if (!imageKeys.value.some(key => key.id === selectedKeyId.value)) selectedKeyId.value = imageKeys.value[0]?.id || 0
  await loadAvailableModels()
}

async function loadAvailableModels() {
  const key = selectedKey.value
  const requestId = ++modelRequestSeq
  availableModels.value = []
  form.value.model = ''
  modelLoadError.value = ''
  if (!key) return

  loadingModels.value = true
  try {
    const models = await listImageGenerationModels(key.key, key.group?.platform)
    if (requestId !== modelRequestSeq) return
    availableModels.value = models
    form.value.model = models[0] || ''
  } catch (error) {
    if (requestId !== modelRequestSeq) return
    modelLoadError.value = error instanceof Error ? error.message : t('imageGeneration.loadModelsFailed')
  } finally {
    if (requestId === modelRequestSeq) loadingModels.value = false
  }
}

function onSourceImage(event: Event) { const file = (event.target as HTMLInputElement).files?.[0]; if (!file) return; if (sourcePreview.value) URL.revokeObjectURL(sourcePreview.value); sourceImage.value = file; sourcePreview.value = URL.createObjectURL(file) }
function clearSourceImage() { if (sourcePreview.value) URL.revokeObjectURL(sourcePreview.value); sourceImage.value = null; sourcePreview.value = '' }
function clearResult() { images.value = []; errorMessage.value = ''; downloadError.value = '' }
async function downloadImage(image: GeneratedImage) {
  downloadError.value = ''
  try {
    await downloadGeneratedImage(image, generatedImageFilename(image, form.value.output_format), selectedKey.value?.key)
  } catch {
    downloadError.value = t('imageGeneration.downloadFailed')
  }
}
async function submit() { if (!canSubmit.value || !selectedKey.value) return; submitting.value = true; errorMessage.value = ''; downloadError.value = ''; images.value = []; try { const result = await generateImages(selectedKey.value.key, { ...form.value, model: form.value.model.trim(), prompt: form.value.prompt.trim() }, mode.value === 'edit' ? sourceImage.value : null); images.value = result.images; if (!images.value.length) errorMessage.value = t('imageGeneration.noImagesReturned') } catch (error) { errorMessage.value = error instanceof Error ? error.message : t('common.unknownError') } finally { submitting.value = false } }
</script>

<style scoped>
.field { display: flex; flex-direction: column; gap: 0.45rem; }
.field > span { font-size: 0.875rem; font-weight: 500; color: rgb(55 65 81); }
.dark .field > span { color: rgb(209 213 219); }
.field small { font-size: 0.75rem; color: rgb(156 163 175); text-align: right; }
</style>
