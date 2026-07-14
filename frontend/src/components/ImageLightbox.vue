<script setup lang="ts">
defineProps<{ src: string }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <div class="lightbox" @click="emit('close')">
    <button type="button" class="close-btn" aria-label="关闭" @click="emit('close')">×</button>
    <img :src="src" alt="预览" class="lightbox-img" @click.stop />
  </div>
</template>

<style scoped>
.lightbox {
  position: fixed;
  inset: 0;
  z-index: 400;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.85);
  padding: var(--space-4);
  animation: lightbox-in var(--transition-base) ease;
}

.lightbox-img {
  max-width: min(96vw, 900px);
  max-height: 90vh;
  object-fit: contain;
  border-radius: var(--radius-sm);
  animation: lightbox-img-in var(--transition-base) ease;
}

.close-btn {
  position: absolute;
  top: var(--space-4);
  right: var(--space-4);
  width: 40px;
  height: 40px;
  font-size: 1.5rem;
  color: #fff;
  background: rgba(255, 255, 255, 0.15);
  border-radius: 50%;
  transition: background var(--transition-fast);
}

.close-btn:hover {
  background: rgba(255, 255, 255, 0.28);
}

@keyframes lightbox-in {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

@keyframes lightbox-img-in {
  from {
    opacity: 0;
    transform: scale(0.96);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}
</style>
