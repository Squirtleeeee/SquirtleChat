<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'

const props = defineProps<{ file: File }>()
const emit = defineEmits<{ confirm: [blob: Blob] }>()

const canvasRef = ref<HTMLCanvasElement | null>(null)
const img = new Image()
const scale = ref(1)
const offsetX = ref(0)
const offsetY = ref(0)
const dragging = ref(false)
const lastX = ref(0)
const lastY = ref(0)
const size = 280

onMounted(() => {
  img.onload = () => {
    const fit = Math.max(size / img.width, size / img.height)
    scale.value = fit
    offsetX.value = (size - img.width * fit) / 2
    offsetY.value = (size - img.height * fit) / 2
    draw()
  }
  img.src = URL.createObjectURL(props.file)
})

onUnmounted(() => URL.revokeObjectURL(img.src))

watch([scale, offsetX, offsetY], draw)

function draw() {
  const c = canvasRef.value
  if (!c || !img.complete) return
  const ctx = c.getContext('2d')
  if (!ctx) return
  ctx.clearRect(0, 0, size, size)
  ctx.save()
  ctx.beginPath()
  ctx.arc(size / 2, size / 2, size / 2, 0, Math.PI * 2)
  ctx.closePath()
  ctx.clip()
  ctx.drawImage(img, offsetX.value, offsetY.value, img.width * scale.value, img.height * scale.value)
  ctx.restore()
  ctx.strokeStyle = 'rgba(255,255,255,0.9)'
  ctx.lineWidth = 2
  ctx.beginPath()
  ctx.arc(size / 2, size / 2, size / 2 - 1, 0, Math.PI * 2)
  ctx.stroke()
}

function onWheel(e: WheelEvent) {
  e.preventDefault()
  const delta = e.deltaY > 0 ? -0.05 : 0.05
  scale.value = Math.min(4, Math.max(0.2, scale.value + delta))
}

function onPointerDown(e: PointerEvent) {
  dragging.value = true
  lastX.value = e.clientX
  lastY.value = e.clientY
}

function onPointerMove(e: PointerEvent) {
  if (!dragging.value) return
  offsetX.value += e.clientX - lastX.value
  offsetY.value += e.clientY - lastY.value
  lastX.value = e.clientX
  lastY.value = e.clientY
}

function onPointerUp() {
  dragging.value = false
}

function confirm() {
  const c = canvasRef.value
  if (!c) return
  c.toBlob((blob) => {
    if (blob) emit('confirm', blob)
  }, 'image/jpeg', 0.92)
}
</script>

<template>
  <div class="cropper">
    <canvas
      ref="canvasRef"
      :width="size"
      :height="size"
      class="crop-canvas"
      @wheel="onWheel"
      @pointerdown="onPointerDown"
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @pointerleave="onPointerUp"
    />
    <p class="crop-hint">滚轮缩放，拖动调整位置</p>
    <button type="button" class="btn btn-primary btn-block" @click="confirm">使用此头像</button>
  </div>
</template>

<style scoped>
.cropper {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-3);
}

.crop-canvas {
  border-radius: 50%;
  cursor: grab;
  touch-action: none;
  box-shadow: var(--shadow-md);
}

.crop-canvas:active {
  cursor: grabbing;
}

.crop-hint {
  margin: 0;
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}
</style>
