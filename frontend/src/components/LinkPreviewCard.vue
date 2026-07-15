<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import http, { unwrapApiData } from '../api/http'
import { parseError } from '../api/errors'

export type LinkPreviewData = {
  url: string
  title?: string
  description?: string
  image?: string
  site_name?: string
}

const props = defineProps<{ url: string }>()

const data = ref<LinkPreviewData | null>(null)
const failed = ref(false)
const failMsg = ref('')
const loading = ref(false)

function hostOf(u: string) {
  try {
    return new URL(u).hostname
  } catch {
    return u
  }
}

async function load(refresh = false) {
  if (!props.url) return
  loading.value = true
  failed.value = false
  failMsg.value = ''
  try {
    const { data: res } = await http.get('/link-preview', {
      params: { url: props.url, ...(refresh ? { refresh: '1' } : {}) },
    })
    data.value = unwrapApiData<LinkPreviewData>(res)
  } catch (e) {
    failed.value = true
    data.value = null
    failMsg.value = parseError(e) || '预览失败'
  } finally {
    loading.value = false
  }
}

function retry() {
  void load(true)
}

onMounted(() => load(false))
watch(
  () => props.url,
  () => load(false),
)
</script>

<template>
  <a
    v-if="data && !failed"
    class="link-card"
    :href="data.url"
    target="_blank"
    rel="noopener noreferrer"
    @click.stop
  >
    <img v-if="data.image" class="link-card-img" :src="data.image" alt="" loading="lazy" />
    <div class="link-card-body">
      <div class="link-card-site">{{ data.site_name || hostOf(data.url) }}</div>
      <div class="link-card-title">{{ data.title }}</div>
      <div v-if="data.description" class="link-card-desc">{{ data.description }}</div>
    </div>
  </a>
  <div v-else-if="loading" class="link-card loading">加载预览…</div>
  <div v-else-if="failed" class="link-card failed" @click.stop>
    <div class="link-card-body">
      <div class="link-card-site">{{ hostOf(url) }}</div>
      <div class="link-card-title">无法加载预览</div>
      <div class="link-card-desc">{{ failMsg }}</div>
      <button type="button" class="link-retry" :disabled="loading" @click="retry">重试</button>
    </div>
  </div>
</template>

<style scoped>
.link-card {
  display: flex;
  gap: 10px;
  margin-top: 8px;
  max-width: 320px;
  padding: 8px;
  border-radius: 8px;
  border: 1px solid var(--color-border, #e2e8f0);
  background: rgba(15, 23, 42, 0.03);
  text-decoration: none;
  color: inherit;
  overflow: hidden;
}

.link-card.loading,
.link-card.failed {
  font-size: 12px;
  color: var(--color-text-muted, #64748b);
}

.link-card-img {
  width: 72px;
  height: 72px;
  object-fit: cover;
  border-radius: 6px;
  flex-shrink: 0;
  background: #e2e8f0;
}

.link-card-body {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.link-card-site {
  font-size: 11px;
  color: var(--color-text-muted, #64748b);
}

.link-card-title {
  font-size: 13px;
  font-weight: 600;
  line-height: 1.3;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  color: var(--color-text, inherit);
}

.link-card-desc {
  font-size: 12px;
  color: var(--color-text-secondary, #475569);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.link-retry {
  align-self: flex-start;
  margin-top: 6px;
  padding: 4px 10px;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-primary, #0d9488);
  background: transparent;
  border: 1px solid var(--color-primary, #0d9488);
  border-radius: 6px;
  cursor: pointer;
}

.link-retry:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.link-retry:hover:not(:disabled) {
  background: color-mix(in srgb, var(--color-primary, #0d9488) 10%, transparent);
}
</style>
