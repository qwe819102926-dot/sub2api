<template>
  <AppLayout>
    <div class="guide-page">
      <iframe
        class="guide-frame"
        src="/usage-guide/index.html"
        title="key智中转使用说明"
        loading="eager"
      ></iframe>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
onMounted(() => {
  document.title = 'key智中转使用说明'
})
</script>

<style scoped>
.guide-page {
  height: calc(100vh - 64px);
  min-height: 0;
  overflow: hidden;
  background: #f8fafb;
}

.guide-frame {
  display: block;
  width: 100%;
  height: 100%;
  border: 0;
}
</style>
